// SPDX-License-Identifier: Apache-2.0

package integration

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	gemara "github.com/gemaraproj/go-gemara"
)

//go:embed policy/catalog.json
var catalogJSON []byte

//go:embed policy/common.json
var policyJSON []byte

type Assessment struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	State  string `json:"state"`
	Reason string `json:"reason"`
}

type Report struct {
	Version           string               `json:"schema_version"`
	Engine            string               `json:"engine"`
	SDK               string               `json:"sdk_schema_version"`
	EvidenceKind      string               `json:"evidence_kind"`
	Decision          string               `json:"decision"`
	Authorization     string               `json:"authorization"`
	MutationPermitted bool                 `json:"mutation_permitted"`
	PolicyDigest      string               `json:"policy_sha256"`
	CatalogDigest     string               `json:"catalog_sha256"`
	BindingDigest     string               `json:"binding_sha256"`
	Subject           Subject              `json:"subject"`
	Assessments       []Assessment         `json:"assessments"`
	Log               gemara.EvaluationLog `json:"evaluation_log"`
}

type payload struct{ gemara.EvidenceCollector }

func SharedDigests() (catalog, policy string) {
	var c, p any
	_ = json.Unmarshal(catalogJSON, &c)
	_ = json.Unmarshal(policyJSON, &p)
	return Digest(c), Digest(p)
}

// EvaluateFixture executes real Gemara assessments against explicitly synthetic
// observations. An eligible fixture can never authorize a forge operation.
func EvaluateFixture(b Binding, e Evidence, now time.Time) (Report, error) {
	if e.SourceKind != "synthetic-fixture" {
		return Report{}, fmt.Errorf("live_evidence_not_implemented")
	}
	return evaluateObservation(b, e, now)
}

// EvaluateCandidate checks normalized collector data. The trusted caller owns
// forge authentication and current-revision binding; this API grants no authority.
func EvaluateCandidate(b Binding, e Evidence, now time.Time) (Report, error) {
	if e.SourceKind != "collector-observation" {
		return Report{}, fmt.Errorf("collector_observation_required")
	}
	return evaluateObservation(b, e, now)
}

func evaluateObservation(b Binding, e Evidence, now time.Time) (Report, error) {
	var r Report
	if err := b.Validate(); err != nil {
		return r, err
	}
	if now.IsZero() || !sha.MatchString(e.Subject.Head) || !sha.MatchString(e.Subject.Base) || e.Subject.Change < 1 || e.Subject.Change > 99999999 {
		return r, fmt.Errorf("invalid_subject")
	}
	var cat gemara.ControlCatalog
	var pol gemara.Policy
	if err := Decode(catalogJSON, &cat); err != nil {
		return r, err
	}
	if err := Decode(policyJSON, &pol); err != nil {
		return r, err
	}
	if len(cat.Controls) != 5 || len(pol.Adherence.AssessmentPlans) != 5 {
		return r, fmt.Errorf("incomplete_shared_policy")
	}
	catalogDigest, policyDigest := SharedDigests()
	r = Report{Version: "owner-evaluation/v1", Engine: "gemara", SDK: gemara.SchemaVersion, EvidenceKind: e.SourceKind, Decision: "eligible", Authorization: "none", PolicyDigest: policyDigest, CatalogDigest: catalogDigest, BindingDigest: Digest(b), Subject: e.Subject}
	evidenceType, evidenceRef, evidenceTitle := e.SourceKind, "observation-evidence", "Normalized evidence; source kind recorded in report"
	description := "Normalized candidate evaluation; no authentication or merge authority"
	evaluationMessage := "Normalized observation assessed using the native Gemara SDK; authentication is external"
	targetName := b.Identity.Repository + " candidate observation"
	if e.SourceKind == "synthetic-fixture" {
		evidenceType, evidenceRef, evidenceTitle = "synthetic-observation", "fixture-evidence", "Synthetic normalized evidence"
		description = "Synthetic candidate evaluation; no live authentication or merge authority"
		evaluationMessage = "Synthetic fixture assessed using the native Gemara SDK"
		targetName = b.Identity.Repository + " candidate fixture"
	}
	expected := e.Subject
	expected.Identity = b.Identity
	expected.PolicyDigest = policyDigest
	expected.BindingDigest = Digest(b)
	for _, control := range cat.Controls {
		if len(control.AssessmentRequirements) != 1 {
			return Report{}, fmt.Errorf("invalid_requirement")
		}
		req := control.AssessmentRequirements[0]
		var plan *gemara.AssessmentPlan
		for i := range pol.Adherence.AssessmentPlans {
			if pol.Adherence.AssessmentPlans[i].RequirementId == req.Id {
				if plan != nil {
					return Report{}, fmt.Errorf("ambiguous_plan")
				}
				plan = &pol.Adherence.AssessmentPlans[i]
			}
		}
		if plan == nil || len(plan.EvaluationMethods) != 1 || !plan.EvaluationMethods[0].Required || plan.EvaluationMethods[0].Id != "evaluate-"+control.Id {
			return Report{}, fmt.Errorf("unbound_plan")
		}
		state, reason := check(control.Id, b, e, expected, now)
		if state == gemara.Passed && e.SourceKind == "synthetic-fixture" {
			reason = "satisfied_in_synthetic_fixture"
		}
		c := &gemara.ControlEvaluation{Name: control.Title, Control: gemara.EntryMapping{ReferenceId: cat.Metadata.Id, EntryId: control.Id}}
		p := &payload{}
		ev := map[string]any{"id": control.Id, "type": evidenceType, "collected-at": e.CollectedAt, "payload": map[string]any{"evidence-kind": e.SourceKind, "subject-sha256": Digest(e.Subject), "observation-sha256": Digest(e), "reason": reason}, "source": map[string]any{"reference-id": evidenceRef, "coordinate": control.Id, "digest": "sha256:" + Digest(e)}}
		var nativeEvidence gemara.Evidence
		raw, _ := json.Marshal(ev)
		if err := json.Unmarshal(raw, &nativeEvidence); err != nil {
			return Report{}, err
		}
		p.AddEvidence(nativeEvidence)
		a := c.AddAssessment(req.Id, req.Text, req.Applicability, []gemara.AssessmentStep{gemara.NamedStep(plan.EvaluationMethods[0].Id, func(any) (gemara.Result, string, gemara.ConfidenceLevel) { return state, reason, gemara.Undetermined })})
		a.Requirement.ReferenceId = cat.Metadata.Id
		a.Plan = &gemara.EntryMapping{ReferenceId: pol.Metadata.Id, EntryId: plan.Id}
		a.Run(p)
		c.Result = a.Result
		c.Message = evaluationMessage
		r.Log.Evaluations = append(r.Log.Evaluations, c)
		r.Log.Result = gemara.UpdateAggregateResult(r.Log.Result, a.Result)
		label := "unknown"
		if a.Result == gemara.Passed {
			label = "passed"
		}
		if a.Result == gemara.Failed {
			label = "failed"
		}
		r.Assessments = append(r.Assessments, Assessment{control.Id, control.Title, label, a.Message})
		if a.Result != gemara.Passed {
			r.Decision = "hold"
		}
	}
	meta := map[string]any{"id": "evaluation-" + Digest(map[string]any{"binding": b, "evidence": e, "at": now.UTC().Format(time.RFC3339Nano)}), "type": "EvaluationLog", "gemara-version": "1.5.0", "description": description, "author": map[string]any{"id": "fullsend-gemara-integration", "name": "Fullsend Gemara integration", "type": "Software"}, "mapping-references": []map[string]any{{"id": cat.Metadata.Id, "title": cat.Title, "version": cat.Metadata.Version}, {"id": pol.Metadata.Id, "title": pol.Title, "version": pol.Metadata.Version}, {"id": evidenceRef, "title": evidenceTitle, "version": "1"}}}
	raw, _ := json.Marshal(meta)
	if err := json.Unmarshal(raw, &r.Log.Metadata); err != nil {
		return Report{}, err
	}
	target := map[string]any{"id": "sha256:" + Digest(e.Subject), "name": targetName, "type": "Software"}
	raw, _ = json.Marshal(target)
	if err := json.Unmarshal(raw, &r.Log.Target); err != nil {
		return Report{}, err
	}
	return r, nil
}

func check(id string, b Binding, e Evidence, expected Subject, now time.Time) (gemara.Result, string) {
	if !reflect.DeepEqual(e.Subject, expected) {
		return gemara.Unknown, "wrong_subject_or_policy"
	}
	collected, err := time.Parse(time.RFC3339, e.CollectedAt)
	if err != nil || now.Before(collected) || now.Sub(collected) > time.Duration(b.MaximumAgeSeconds)*time.Second {
		return gemara.Unknown, "stale_or_future_evidence"
	}
	switch id {
	case "OWNER-SCOPE":
		expires, _ := time.Parse(time.RFC3339, b.GrantExpiresAt)
		if !b.GrantEnabled || !now.Before(expires) {
			return gemara.Failed, "owner_grant_inactive"
		}
		if !e.Open || e.Draft {
			return gemara.Failed, "candidate_not_open_ready"
		}
		if len(e.Files) == 0 {
			return gemara.Unknown, "files_missing"
		}
		seen := map[string]bool{}
		for _, f := range e.Files {
			if !validPath(f.Path) || seen[f.Path] || f.Mode != "100644" || f.PreviousPath != "" || f.Kind != "modified" {
				return gemara.Unknown, "unsupported_or_incomplete_change"
			}
			seen[f.Path] = true
			if matches(f.Path, b.GovernedPaths) || !matches(f.Path, b.AllowedPaths) {
				return gemara.Failed, "outside_owner_delegation"
			}
		}
	case "REVIEW-CLEAR":
		return checkReview(e.Review, b.MaximumRisk)
	case "CHECKS-PASS":
		for _, name := range b.RequiredChecks {
			found := 0
			for _, c := range e.Checks {
				if c.Name == name {
					found++
					if c.Head != e.Subject.Head {
						return gemara.Unknown, "check_for_wrong_revision"
					}
					if c.Status != "success" {
						return gemara.Failed, "required_check_not_successful"
					}
				}
			}
			if found != 1 {
				return gemara.Unknown, "required_check_missing_or_ambiguous"
			}
		}
	case "EVIDENCE-CURRENT":
		p := e.Publication
		if p.Workflow != b.ReviewWorkflow || p.Run == "" || p.Attempt < 1 || !hash.MatchString(p.ArtifactDigest) || !reflect.DeepEqual(p.Subject, e.Subject) {
			return gemara.Unknown, "publication_binding_invalid"
		}
	case "PROTECTIONS-ACTIVE":
		p := e.Protections
		expectedMechanism := map[string]string{"github": "merge-queue", "gitlab": "merge-train"}[b.Identity.Forge]
		if !p.ProtectedTarget || !p.RequiredPolicyGate || !p.DiscussionsResolved || p.BypassEnabled || p.Mechanism != expectedMechanism {
			return gemara.Failed, "protected_merge_requirements_not_met"
		}
	default:
		return gemara.Unknown, "unsupported_control"
	}
	return gemara.Passed, "requirements_satisfied_in_observation"
}
