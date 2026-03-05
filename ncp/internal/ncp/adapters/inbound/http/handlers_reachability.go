package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	ncperr "github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/errors"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/intent"
	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/ports/inbound"
)

type ReachabilityHandlers struct {
	Svc inbound.ReachabilityService
}

func (h ReachabilityHandlers) Register(r chi.Router) {
	r.Get("/reachability", h.check)
}

func (h ReachabilityHandlers) check(w http.ResponseWriter, r *http.Request) {
	actor := ActorFrom(r.Context())
	qp := r.URL.Query()

	intentIDStr := strings.TrimSpace(qp.Get("intentId"))
	var intentID *uuid.UUID
	if intentIDStr != "" {
		id, err := uuid.Parse(intentIDStr)
		if err != nil {
			writeError(w, ncperr.InvalidArgument("bad intentId", nil, err))
			return
		}
		intentID = &id
	}

	var revPtr *int
	if rs := qp.Get("revision"); rs != "" {
		if intentID == nil {
			writeError(w, ncperr.InvalidArgument("revision requires intentId", nil, nil))
			return
		}
		n, err := strconv.Atoi(rs)
		if err != nil || n <= 0 {
			writeError(w, ncperr.InvalidArgument("bad revision", nil, err))
			return
		}
		revPtr = &n
	}

	fromNS := qp.Get("fromNamespace")
	toNS := qp.Get("toNamespace")
	toSvc := qp.Get("toService")
	if fromNS == "" || toNS == "" || toSvc == "" {
		writeError(w, ncperr.InvalidArgument(
			"fromNamespace, toNamespace, toService are required",
			map[string]any{"required": []string{"fromNamespace", "toNamespace", "toService"}},
			nil,
		))
		return
	}

	portStr := qp.Get("port")
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		writeError(w, ncperr.InvalidArgument("bad port", map[string]any{
			"port": portStr,
		}, err))
		return
	}

	proto := intent.ProtoTCP
	if p := qp.Get("protocol"); p != "" {
		switch strings.ToUpper(p) {
		case "TCP":
			proto = intent.ProtoTCP
		case "UDP":
			proto = intent.ProtoUDP
		default:
			writeError(w, ncperr.InvalidArgument("bad protocol (TCP|UDP)", map[string]any{
				"protocol": p,
				"allowed":  []string{"TCP", "UDP"},
			}, nil))
			return
		}
	}

	dir := intent.DirEgress
	if d := qp.Get("direction"); d != "" {
		switch strings.ToLower(d) {
		case "egress":
			dir = intent.DirEgress
		case "ingress":
			dir = intent.DirIngress
		case "both":
			dir = intent.DirBoth
		default:
			writeError(w, ncperr.InvalidArgument("bad direction (egress|ingress|both)", map[string]any{
				"direction": d,
				"allowed":   []string{"egress", "ingress", "both"},
			}, nil))
			return
		}
	}

	selector := map[string]string{}
	if s := qp.Get("fromSelector"); s != "" {
		parts := strings.Split(s, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			kv := strings.SplitN(p, "=", 2)
			if len(kv) != 2 || strings.TrimSpace(kv[0]) == "" {
				writeError(w, ncperr.InvalidArgument("bad fromSelector format (k=v,k2=v2)", map[string]any{
					"fromSelector": s,
				}, nil))
				return
			}
			selector[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}

	q := inbound.ReachabilityQuery{
		Revision:      revPtr,
		FromNamespace: fromNS,
		FromSelector:  selector,
		ToNamespace:   toNS,
		ToService:     toSvc,
		Port:          port,
		Protocol:      proto,
		Direction:     dir,
	}

	if intentID != nil {
		q.IntentID = *intentID
	}

	res, err := h.Svc.CheckReachability(r.Context(), actor, q)
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}
