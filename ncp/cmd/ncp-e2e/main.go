package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/bootstrap/config"
	netv1 "k8s.io/api/networking/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type cfg struct {
	APIURL     string
	Token      string
	DBDSN      string
	Kubeconfig string
	KubeCtx    string
	Namespace  string
	Timeout    time.Duration
}

type runner struct {
	cfg  cfg
	http *http.Client
	db   *pgxpool.Pool
	kube kubernetes.Interface
}

type scenarioResult struct {
	Name   string
	Intent string
	Plan   string
	Detail string
}

func main() {
	config.LoadDotEnv(".env")

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: ncp-e2e <run|summary> [flags]")
		os.Exit(2)
	}

	cmd := os.Args[1]
	switch cmd {
	case "run":
		runCmd(os.Args[2:])
	case "summary":
		summaryCmd(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", cmd)
		os.Exit(2)
	}
}

func runCmd(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	scenarios := fs.String("scenarios", "happy,ttl-delete,ttl-rollback,idempotency", "comma-separated scenarios")
	_ = fs.Parse(args)

	cfg, err := loadCfg()
	fatalIfErr(err)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	r, err := newRunner(ctx, cfg)
	fatalIfErr(err)
	defer r.close()

	fatalIfErr(r.preflight(ctx))

	plan := parseScenarios(*scenarios)
	if len(plan) == 0 {
		fatalIfErr(errors.New("no scenarios selected"))
	}

	fmt.Printf("ncp-e2e run: scenarios=%s\n", strings.Join(plan, ","))

	results := make([]scenarioResult, 0, len(plan))
	for _, name := range plan {
		fmt.Printf("\n scenario: %s \n", name)
		var res scenarioResult
		switch name {
		case "happy":
			res, err = r.scenarioHappy(ctx)
		case "ttl-delete":
			res, err = r.scenarioTTLDelete(ctx)
		case "ttl-rollback":
			res, err = r.scenarioTTLRollback(ctx)
		case "idempotency":
			res, err = r.scenarioIdempotency(ctx)
		default:
			err = fmt.Errorf("unknown scenario %q", name)
		}
		if err != nil {
			fatalIfErr(fmt.Errorf("scenario %s failed: %w", name, err))
		}
		results = append(results, res)
		fmt.Printf("ok: %s\n", res.Detail)
	}

	fmt.Println("\n e2e summary ")
	for _, res := range results {
		fmt.Printf("- %s: intent=%s plan=%s %s\n", res.Name, res.Intent, res.Plan, res.Detail)
	}
}

func summaryCmd(args []string) {
	fs := flag.NewFlagSet("summary", flag.ExitOnError)
	limit := fs.Int("limit", 20, "rows per section")
	_ = fs.Parse(args)

	cfg, err := loadCfg()
	fatalIfErr(err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	r, err := newRunner(ctx, cfg)
	fatalIfErr(err)
	defer r.close()

	fmt.Println(" plans ")
	rows, err := r.db.Query(ctx, `
		SELECT id::text, intent_id::text, status, COALESCE(apply_job_id::text,''), created_at
		FROM plans
		ORDER BY created_at DESC
		LIMIT $1
	`, *limit)
	fatalIfErr(err)
	for rows.Next() {
		var id, intentID, status, applyJob string
		var createdAt time.Time
		fatalIfErr(rows.Scan(&id, &intentID, &status, &applyJob, &createdAt))
		fmt.Printf("%s intent=%s status=%s applyJob=%s at=%s\n", id, intentID, status, emptyDash(applyJob), createdAt.UTC().Format(time.RFC3339))
	}
	fatalIfErr(rows.Err())
	rows.Close()

	fmt.Println("\n jobs ")
	rows, err = r.db.Query(ctx, `
		SELECT id::text, kind, status, COALESCE(payload->>'planId',''), created_at
		FROM jobs
		ORDER BY created_at DESC
		LIMIT $1
	`, *limit)
	fatalIfErr(err)
	for rows.Next() {
		var id, kind, status, planID string
		var createdAt time.Time
		fatalIfErr(rows.Scan(&id, &kind, &status, &planID, &createdAt))
		fmt.Printf("%s kind=%s status=%s plan=%s at=%s\n", id, kind, status, emptyDash(planID), createdAt.UTC().Format(time.RFC3339))
	}
	fatalIfErr(rows.Err())
	rows.Close()

	fmt.Println("\n audit_log ")
	rows, err = r.db.Query(ctx, `
		SELECT id, action, entity_type, entity_id, at
		FROM audit_log
		ORDER BY id DESC
		LIMIT $1
	`, *limit)
	fatalIfErr(err)
	for rows.Next() {
		var id int64
		var action, entityType, entityID string
		var at time.Time
		fatalIfErr(rows.Scan(&id, &action, &entityType, &entityID, &at))
		fmt.Printf("%d action=%s entity=%s/%s at=%s\n", id, action, entityType, entityID, at.UTC().Format(time.RFC3339))
	}
	fatalIfErr(rows.Err())
	rows.Close()
}

func loadCfg() (cfg, error) {
	apiURL := getenvDefault("NCP_API_URL", "http://127.0.0.1:8080")
	token := getenvDefault("NCP_DEV_TOKEN", "devtoken")
	dsn := strings.TrimSpace(os.Getenv("NCP_DB_DSN"))
	kubeconfig := strings.TrimSpace(os.Getenv("NCP_KUBECONFIG"))
	kubeCtx := strings.TrimSpace(os.Getenv("NCP_KUBE_CONTEXT"))
	ns := getenvDefault("NCP_E2E_NAMESPACE", "default")
	timeoutRaw := getenvDefault("NCP_E2E_TIMEOUT", "6m")

	if dsn == "" {
		return cfg{}, errors.New("NCP_DB_DSN is required")
	}
	if kubeconfig == "" {
		kubeconfig = "~/.kube/config"
	}

	timeout, err := time.ParseDuration(timeoutRaw)
	if err != nil {
		return cfg{}, fmt.Errorf("bad NCP_E2E_TIMEOUT: %w", err)
	}

	return cfg{
		APIURL:     strings.TrimRight(apiURL, "/"),
		Token:      token,
		DBDSN:      dsn,
		Kubeconfig: kubeconfig,
		KubeCtx:    kubeCtx,
		Namespace:  ns,
		Timeout:    timeout,
	}, nil
}

func newRunner(ctx context.Context, cfg cfg) (*runner, error) {
	db, err := pgxpool.New(ctx, cfg.DBDSN)
	if err != nil {
		return nil, fmt.Errorf("connect db: %w", err)
	}

	kube, err := buildKubeClient(cfg.Kubeconfig, cfg.KubeCtx)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("build kube client: %w", err)
	}

	return &runner{
		cfg:  cfg,
		http: &http.Client{Timeout: 15 * time.Second},
		db:   db,
		kube: kube,
	}, nil
}

func (r *runner) close() {
	if r.db != nil {
		r.db.Close()
	}
}

func (r *runner) preflight(ctx context.Context) error {
	if err := r.waitHTTPReady(ctx, 60*time.Second); err != nil {
		return err
	}

	if err := r.db.Ping(ctx); err != nil {
		return fmt.Errorf("db ping failed: %w", err)
	}

	_, err := r.kube.NetworkingV1().NetworkPolicies(r.cfg.Namespace).List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return fmt.Errorf("k8s access failed for namespace %q: %w", r.cfg.Namespace, err)
	}

	fmt.Printf("preflight ok: api=%s db=ok k8s namespace=%s\n", r.cfg.APIURL, r.cfg.Namespace)
	return nil
}

func (r *runner) waitHTTPReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("api %s is not ready within %s", r.cfg.APIURL, timeout)
		}

		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, r.cfg.APIURL+"/health", nil)
		resp, err := r.http.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(1 * time.Second)
	}
}

type intentCreateReq struct {
	Key       string            `json:"key"`
	Title     string            `json:"title"`
	OwnerTeam string            `json:"ownerTeam"`
	Labels    map[string]string `json:"labels,omitempty"`
}

type intentCreateResp struct {
	ID string `json:"id"`
}

type createRevisionReq struct {
	Spec          map[string]any `json:"spec"`
	TicketRef     string         `json:"ticketRef"`
	Justification string         `json:"justification"`
	TTLSeconds    *int           `json:"ttlSeconds,omitempty"`
}

type createRevisionResp struct {
	Revision int `json:"revision"`
}

type createPlanResp struct {
	PlanID string `json:"planId"`
}

type planResp struct {
	ID        string         `json:"id"`
	IntentID  string         `json:"intentId"`
	Revision  int64          `json:"revisionId"`
	Status    string         `json:"status"`
	Artifacts map[string]any `json:"artifacts"`
}

func (r *runner) createIntent(ctx context.Context, req intentCreateReq) (uuid.UUID, error) {
	var out intentCreateResp
	if err := r.apiJSON(ctx, http.MethodPost, "/intents", req, &out); err != nil {
		return uuid.Nil, err
	}
	return uuid.Parse(out.ID)
}

func (r *runner) createRevision(ctx context.Context, intentID uuid.UUID, req createRevisionReq) (int, error) {
	var out createRevisionResp
	if err := r.apiJSON(ctx, http.MethodPost, "/intents/"+intentID.String()+"/revisions", req, &out); err != nil {
		return 0, err
	}
	if out.Revision <= 0 {
		return 0, errors.New("create revision returned invalid revision")
	}
	return out.Revision, nil
}

func (r *runner) createPlan(ctx context.Context, intentID uuid.UUID, revision int) (uuid.UUID, error) {
	var out createPlanResp
	path := fmt.Sprintf("/intents/%s/plan?revision=%d", intentID.String(), revision)
	if err := r.apiJSON(ctx, http.MethodPost, path, nil, &out); err != nil {
		return uuid.Nil, err
	}
	return uuid.Parse(out.PlanID)
}

func (r *runner) applyPlan(ctx context.Context, planID uuid.UUID) error {
	return r.apiJSON(ctx, http.MethodPost, "/plans/"+planID.String()+"/apply", nil, nil)
}

func (r *runner) getPlan(ctx context.Context, planID uuid.UUID) (planResp, error) {
	var out planResp
	if err := r.apiJSON(ctx, http.MethodGet, "/plans/"+planID.String(), nil, &out); err != nil {
		return planResp{}, err
	}
	return out, nil
}

func (r *runner) apiJSON(ctx context.Context, method, path string, body any, out any) error {
	var payload io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, r.cfg.APIURL+path, payload)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.cfg.Token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := r.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s -> %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(data)))
	}

	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

func (r *runner) scenarioHappy(ctx context.Context) (scenarioResult, error) {
	suffix := uniqueSuffix()
	app := "e2e-happy-" + suffix
	intentID, rev, planID, plan, err := r.createAndApply(ctx, app, 443, nil)
	if err != nil {
		return scenarioResult{}, err
	}

	ns, name, err := extractAppliedRef(plan)
	if err != nil {
		return scenarioResult{}, err
	}

	if err := r.waitPolicyPort(ctx, ns, name, 443, true, 60*time.Second); err != nil {
		return scenarioResult{}, err
	}
	if err := r.waitAudit(ctx, "apply_done", "plan", planID.String(), 30*time.Second); err != nil {
		return scenarioResult{}, err
	}

	return scenarioResult{
		Name:   "happy",
		Intent: intentID.String(),
		Plan:   planID.String(),
		Detail: fmt.Sprintf("revision=%d policy=%s/%s port=443", rev, ns, name),
	}, nil
}

func (r *runner) scenarioTTLDelete(ctx context.Context) (scenarioResult, error) {
	ttl := 60
	suffix := uniqueSuffix()
	app := "e2e-ttl-del-" + suffix
	intentID, rev, planID, plan, err := r.createAndApply(ctx, app, 9443, &ttl)
	if err != nil {
		return scenarioResult{}, err
	}

	ns, name, err := extractAppliedRef(plan)
	if err != nil {
		return scenarioResult{}, err
	}

	notAfter, err := r.getRevisionNotAfter(ctx, intentID, rev)
	if err != nil {
		return scenarioResult{}, err
	}
	if notAfter == nil {
		return scenarioResult{}, errors.New("revision.not_after was not set after apply")
	}

	fmt.Printf("forcing revision %d expired (not_after was %s)\n", rev, notAfter.UTC().Format(time.RFC3339))
	if err := r.forceRevisionExpired(ctx, intentID, rev); err != nil {
		return scenarioResult{}, err
	}

	if _, err := r.waitPlanStatus(ctx, planID, "expired", 90*time.Second); err != nil {
		return scenarioResult{}, err
	}
	if err := r.waitPolicyDeleted(ctx, ns, name, 90*time.Second); err != nil {
		return scenarioResult{}, err
	}
	if err := r.waitAudit(ctx, "ttl_expired", "plan", planID.String(), 60*time.Second); err != nil {
		return scenarioResult{}, err
	}
	if err := r.waitAudit(ctx, "ttl_deleted", "plan", planID.String(), 60*time.Second); err != nil {
		return scenarioResult{}, err
	}

	return scenarioResult{
		Name:   "ttl-delete",
		Intent: intentID.String(),
		Plan:   planID.String(),
		Detail: fmt.Sprintf("revision=%d policy deleted=%s/%s", rev, ns, name),
	}, nil
}

func (r *runner) scenarioTTLRollback(ctx context.Context) (scenarioResult, error) {
	suffix := uniqueSuffix()
	app := "e2e-ttl-rb-" + suffix
	key := "e2e.ttl-rb." + suffix

	intentID, err := r.createIntent(ctx, intentCreateReq{
		Key:       key,
		Title:     "e2e ttl rollback",
		OwnerTeam: "platform",
		Labels: map[string]string{
			"e2e": "true",
		},
	})
	if err != nil {
		return scenarioResult{}, err
	}

	rev1, err := r.createRevision(ctx, intentID, createRevisionReq{
		Spec:          buildSpec(r.cfg.Namespace, app, 443),
		TicketRef:     "E2E-TTL-RB-1",
		Justification: "e2e baseline",
	})
	if err != nil {
		return scenarioResult{}, err
	}
	plan1, err := r.createPlan(ctx, intentID, rev1)
	if err != nil {
		return scenarioResult{}, err
	}
	if err := r.applyPlan(ctx, plan1); err != nil {
		return scenarioResult{}, err
	}
	p1, err := r.waitPlanStatus(ctx, plan1, "applied", 90*time.Second)
	if err != nil {
		return scenarioResult{}, err
	}
	ns, name, err := extractAppliedRef(p1)
	if err != nil {
		return scenarioResult{}, err
	}
	if err := r.waitPolicyPort(ctx, ns, name, 443, true, 60*time.Second); err != nil {
		return scenarioResult{}, err
	}

	ttl := 60
	rev2, err := r.createRevision(ctx, intentID, createRevisionReq{
		Spec:          buildSpec(r.cfg.Namespace, app, 8443),
		TicketRef:     "E2E-TTL-RB-2",
		Justification: "e2e ttl",
		TTLSeconds:    &ttl,
	})
	if err != nil {
		return scenarioResult{}, err
	}
	plan2, err := r.createPlan(ctx, intentID, rev2)
	if err != nil {
		return scenarioResult{}, err
	}
	if err := r.applyPlan(ctx, plan2); err != nil {
		return scenarioResult{}, err
	}
	p2, err := r.waitPlanStatus(ctx, plan2, "applied", 90*time.Second)
	if err != nil {
		return scenarioResult{}, err
	}
	ns2, name2, err := extractAppliedRef(p2)
	if err != nil {
		return scenarioResult{}, err
	}
	if err := r.waitPolicyPort(ctx, ns2, name2, 8443, true, 60*time.Second); err != nil {
		return scenarioResult{}, err
	}

	if err := r.forceRevisionExpired(ctx, intentID, rev2); err != nil {
		return scenarioResult{}, err
	}
	if _, err := r.waitPlanStatus(ctx, plan2, "expired", 90*time.Second); err != nil {
		return scenarioResult{}, err
	}
	if err := r.waitPolicyPort(ctx, ns2, name2, 443, true, 90*time.Second); err != nil {
		return scenarioResult{}, err
	}
	if err := r.waitPolicyPort(ctx, ns2, name2, 8443, false, 90*time.Second); err != nil {
		return scenarioResult{}, err
	}
	if err := r.waitAudit(ctx, "rollback_done", "intent", intentID.String(), 60*time.Second); err != nil {
		return scenarioResult{}, err
	}

	return scenarioResult{
		Name:   "ttl-rollback",
		Intent: intentID.String(),
		Plan:   plan2.String(),
		Detail: fmt.Sprintf("rev1=%d rev2=%d rollback policy=%s/%s", rev1, rev2, ns2, name2),
	}, nil
}

func (r *runner) scenarioIdempotency(ctx context.Context) (scenarioResult, error) {
	suffix := uniqueSuffix()
	app := "e2e-idem-" + suffix
	intentID, _, planID, plan, err := r.createAndApply(ctx, app, 443, nil)
	if err != nil {
		return scenarioResult{}, err
	}
	ns, name, err := extractAppliedRef(plan)
	if err != nil {
		return scenarioResult{}, err
	}

	firstApplyJob, err := r.getPlanApplyJobID(ctx, planID)
	if err != nil {
		return scenarioResult{}, err
	}
	if firstApplyJob == nil {
		return scenarioResult{}, errors.New("plan.apply_job_id is nil after first apply")
	}

	jobsBefore, err := r.countApplyJobsByPlan(ctx, planID)
	if err != nil {
		return scenarioResult{}, err
	}

	if err := r.applyPlan(ctx, planID); err != nil {
		return scenarioResult{}, err
	}
	time.Sleep(2 * time.Second)

	after, err := r.getPlan(ctx, planID)
	if err != nil {
		return scenarioResult{}, err
	}
	if after.Status != "applied" {
		return scenarioResult{}, fmt.Errorf("plan status changed after re-apply: %s", after.Status)
	}

	secondApplyJob, err := r.getPlanApplyJobID(ctx, planID)
	if err != nil {
		return scenarioResult{}, err
	}
	if secondApplyJob == nil || *secondApplyJob != *firstApplyJob {
		return scenarioResult{}, fmt.Errorf("apply_job_id changed after re-apply: before=%v after=%v", firstApplyJob, secondApplyJob)
	}

	jobsAfter, err := r.countApplyJobsByPlan(ctx, planID)
	if err != nil {
		return scenarioResult{}, err
	}
	if jobsAfter != jobsBefore {
		return scenarioResult{}, fmt.Errorf("apply job count changed after re-apply: before=%d after=%d", jobsBefore, jobsAfter)
	}

	if err := r.waitPolicyPort(ctx, ns, name, 443, true, 30*time.Second); err != nil {
		return scenarioResult{}, err
	}

	return scenarioResult{
		Name:   "idempotency",
		Intent: intentID.String(),
		Plan:   planID.String(),
		Detail: fmt.Sprintf("apply_job_id=%s jobs=%d", (*firstApplyJob).String(), jobsAfter),
	}, nil
}

func (r *runner) createAndApply(ctx context.Context, app string, port int, ttl *int) (uuid.UUID, int, uuid.UUID, planResp, error) {
	suffix := uniqueSuffix()
	intentID, err := r.createIntent(ctx, intentCreateReq{
		Key:       "e2e." + app + "." + suffix,
		Title:     "e2e " + app,
		OwnerTeam: "platform",
		Labels: map[string]string{
			"e2e": "true",
		},
	})
	if err != nil {
		return uuid.Nil, 0, uuid.Nil, planResp{}, err
	}

	rev, err := r.createRevision(ctx, intentID, createRevisionReq{
		Spec:          buildSpec(r.cfg.Namespace, app, port),
		TicketRef:     "E2E-" + strings.ToUpper(app),
		Justification: "e2e",
		TTLSeconds:    ttl,
	})
	if err != nil {
		return uuid.Nil, 0, uuid.Nil, planResp{}, err
	}

	planID, err := r.createPlan(ctx, intentID, rev)
	if err != nil {
		return uuid.Nil, 0, uuid.Nil, planResp{}, err
	}

	if err := r.applyPlan(ctx, planID); err != nil {
		return uuid.Nil, 0, uuid.Nil, planResp{}, err
	}

	plan, err := r.waitPlanStatus(ctx, planID, "applied", 90*time.Second)
	if err != nil {
		return uuid.Nil, 0, uuid.Nil, planResp{}, err
	}

	return intentID, rev, planID, plan, nil
}

func (r *runner) waitPlanStatus(ctx context.Context, planID uuid.UUID, status string, timeout time.Duration) (planResp, error) {
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return planResp{}, fmt.Errorf("wait plan %s status=%s timed out", planID.String(), status)
		}
		plan, err := r.getPlan(ctx, planID)
		if err == nil {
			if plan.Status == status {
				return plan, nil
			}
			if strings.HasSuffix(plan.Status, "failed") {
				return planResp{}, fmt.Errorf("plan %s entered failed state: %s", planID.String(), plan.Status)
			}
		}
		time.Sleep(1 * time.Second)
	}
}

func extractAppliedRef(plan planResp) (string, string, error) {
	k8sAny, ok := plan.Artifacts["k8s"]
	if !ok {
		return "", "", errors.New("plan artifacts missing k8s")
	}
	k8sMap, ok := k8sAny.(map[string]any)
	if !ok {
		return "", "", errors.New("plan artifacts.k8s has invalid shape")
	}
	appliedAny, ok := k8sMap["applied"]
	if !ok {
		return "", "", errors.New("plan artifacts.k8s.applied is missing")
	}
	appliedMap, ok := appliedAny.(map[string]any)
	if !ok {
		return "", "", errors.New("plan artifacts.k8s.applied has invalid shape")
	}
	ns, _ := appliedMap["namespace"].(string)
	name, _ := appliedMap["name"].(string)
	if ns == "" || name == "" {
		return "", "", errors.New("plan artifacts.k8s.applied.namespace/name is empty")
	}
	return ns, name, nil
}

func (r *runner) waitPolicyPort(ctx context.Context, namespace, name string, port int32, expected bool, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			if expected {
				return fmt.Errorf("policy %s/%s did not expose port %d in time", namespace, name, port)
			}
			return fmt.Errorf("policy %s/%s still exposes port %d", namespace, name, port)
		}

		np, err := r.kube.NetworkingV1().NetworkPolicies(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				if expected {
					time.Sleep(1 * time.Second)
					continue
				}
				return nil
			}
			return err
		}

		has := policyHasPort(np, port)
		if has == expected {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
}

func (r *runner) waitPolicyDeleted(ctx context.Context, namespace, name string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("policy %s/%s still exists", namespace, name)
		}
		_, err := r.kube.NetworkingV1().NetworkPolicies(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		time.Sleep(1 * time.Second)
	}
}

func policyHasPort(np *netv1.NetworkPolicy, port int32) bool {
	for _, rule := range np.Spec.Egress {
		for _, p := range rule.Ports {
			if p.Port != nil && p.Port.IntVal == port {
				return true
			}
		}
	}
	for _, rule := range np.Spec.Ingress {
		for _, p := range rule.Ports {
			if p.Port != nil && p.Port.IntVal == port {
				return true
			}
		}
	}
	return false
}

func (r *runner) getRevisionNotAfter(ctx context.Context, intentID uuid.UUID, revision int) (*time.Time, error) {
	var out *time.Time
	err := r.db.QueryRow(ctx, `
		SELECT not_after
		FROM intent_revisions
		WHERE intent_id=$1 AND revision=$2
	`, intentID, revision).Scan(&out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *runner) forceRevisionExpired(ctx context.Context, intentID uuid.UUID, revision int) error {
	cmd, err := r.db.Exec(ctx, `
		UPDATE intent_revisions
		SET not_after = now() - interval '2 second'
		WHERE intent_id=$1 AND revision=$2
	`, intentID, revision)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() != 1 {
		return fmt.Errorf("cannot force expire revision: intent=%s revision=%d", intentID.String(), revision)
	}
	return nil
}

func (r *runner) waitAudit(ctx context.Context, action, entityType, entityID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("audit action %s for %s/%s not found", action, entityType, entityID)
		}
		ok, err := r.hasAuditAction(ctx, action, entityType, entityID)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
}

func (r *runner) hasAuditAction(ctx context.Context, action, entityType, entityID string) (bool, error) {
	var n int
	err := r.db.QueryRow(ctx, `
		SELECT count(1)
		FROM audit_log
		WHERE action=$1 AND entity_type=$2 AND entity_id=$3
	`, action, entityType, entityID).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (r *runner) getPlanApplyJobID(ctx context.Context, planID uuid.UUID) (*uuid.UUID, error) {
	var id *uuid.UUID
	err := r.db.QueryRow(ctx, `SELECT apply_job_id FROM plans WHERE id=$1`, planID).Scan(&id)
	if err != nil {
		return nil, err
	}
	return id, nil
}

func (r *runner) countApplyJobsByPlan(ctx context.Context, planID uuid.UUID) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `
		SELECT count(1)
		FROM jobs
		WHERE kind='apply_plan' AND payload->>'planId'=$1
	`, planID.String()).Scan(&n)
	return n, err
}

func buildSpec(namespace, app string, port int) map[string]any {
	return map[string]any{
		"envs": []string{"dev"},
		"owner": map[string]any{
			"team": "platform",
		},
		"subject": map[string]any{
			"cluster":   "local",
			"namespace": namespace,
			"selector": map[string]any{
				"app": app,
			},
		},
		"destinations": []map[string]any{
			{
				"type": "service",
				"service": map[string]any{
					"cluster":   "local",
					"namespace": namespace,
					"name":      "svc-b",
				},
			},
		},
		"rules": []map[string]any{
			{
				"direction": "egress",
				"protocol":  "TCP",
				"ports":     []int{port},
			},
		},
		"constraints": map[string]any{
			"approvalRequired": false,
		},
	}
}

func buildKubeClient(kubeconfig, kubeContext string) (kubernetes.Interface, error) {
	loading := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		expanded, err := expandPath(kubeconfig)
		if err != nil {
			return nil, err
		}
		loading.ExplicitPath = expanded
	}

	overrides := &clientcmd.ConfigOverrides{}
	if kubeContext != "" {
		overrides.CurrentContext = kubeContext
	}

	restCfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loading, overrides).ClientConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(restCfg)
}

func expandPath(p string) (string, error) {
	if p == "" {
		return "", nil
	}
	if p == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return home, nil
	}
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, p[2:]), nil
	}
	return p, nil
}

func parseScenarios(raw string) []string {
	parts := strings.Split(raw, ",")
	set := map[string]struct{}{}
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s == "" {
			continue
		}
		set[s] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	order := []string{"happy", "ttl-delete", "ttl-rollback", "idempotency"}
	idx := map[string]int{}
	for i, s := range order {
		idx[s] = i
	}
	sort.SliceStable(out, func(i, j int) bool {
		ii, okI := idx[out[i]]
		jj, okJ := idx[out[j]]
		if okI && okJ {
			return ii < jj
		}
		if okI {
			return true
		}
		if okJ {
			return false
		}
		return out[i] < out[j]
	})
	return out
}

func uniqueSuffix() string {
	return strings.ReplaceAll(uuid.New().String()[:8], "-", "")
}

func getenvDefault(key, def string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	return v
}

func emptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func fatalIfErr(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
