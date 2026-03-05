package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/reybrally/NetControl-Plane/ncp/internal/ncp/domain/intent"
)

type CompileResult struct {
	SpecHash  string
	Artifacts map[string]any
	Diff      map[string]any
	Blast     map[string]any
}

func Compile(spec intent.Spec) (CompileResult, error) {
	b, _ := json.Marshal(spec)
	h := sha256.Sum256(b)
	specHash := hex.EncodeToString(h[:])

	k8s := map[string]any{
		"kind":         "NetworkPolicy",
		"cluster":      spec.Subject.Cluster,
		"namespace":    spec.Subject.Namespace,
		"podSelector":  spec.Subject.Selector,
		"destinations": spec.Destinations,
		"rules":        spec.Rules,
	}

	aws := map[string]any{
		"kind":         "SecurityGroupChangeSet",
		"envs":         spec.Envs,
		"fromSelector": spec.Subject.Selector,
		"destinations": spec.Destinations,
		"rules":        spec.Rules,
	}

	artifacts := map[string]any{"k8s": k8s, "aws": aws}

	diff := map[string]any{"summary": "mvp: no provider discover yet"}
	blast := map[string]any{"workloads": 1, "namespaces": 1}

	return CompileResult{SpecHash: specHash, Artifacts: artifacts, Diff: diff, Blast: blast}, nil
}
