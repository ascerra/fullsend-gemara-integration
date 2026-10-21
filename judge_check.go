// SPDX-License-Identifier: Apache-2.0

package integration

import (
	_ "embed"
	"fmt"
	"time"

	gemara "github.com/gemaraproj/go-gemara"
)

//go:embed policy/judge-catalog.json
var judgeCatalogJSON []byte

//go:embed policy/judge-policy.json
var judgePolicyJSON []byte

// JudgePolicy is trusted consumer configuration. Basis points avoid rounding.
type JudgePolicy struct {
	Version                   string `json:"schema_version"`
	Scorer                    string `json:"scorer"`
	SourceID                  string `json:"source_id"`
	ExperimentID              string `json:"experiment_id"`
	LookbackDays              int    `json:"lookback_days"`
	MinimumSamples            int    `json:"minimum_samples"`
	MinimumBasisPoints        int    `json:"minimum_basis_points"`
	MaximumSnapshotAgeSeconds int    `json:"maximum_snapshot_age_seconds"`
	MaximumScoreAgeSeconds    int    `json:"maximum_score_age_seconds"`
}

// JudgeObservation is aggregate data, not proof of source authentication.
type JudgeObservation struct {
	Version              string `json:"schema_version"`
	Scorer               string `json:"scorer"`
	SourceID             string `json:"source_id"`
	ExperimentID         string `json:"experiment_id"`
	Cohort               string `json:"cohort"`
	WindowStart          string `json:"window_start"`
	WindowEnd            string `json:"window_end"`
	CollectedAt          string `json:"collected_at"`
	LatestScoreAt        string `json:"latest_score_at"`
	Passed               int    `json:"passed"`
	Total                int    `json:"total"`
	Invalid              int    `json:"invalid"`
	ExcludedOtherSources int    `json:"excluded_other_sources"`
	Complete             bool   `json:"complete"`
	RecordsDigest        string `json:"records_sha256"`
	Authorization        string `json:"authorization"`
	MutationPermitted    bool   `json:"mutation_permitted"`
}

type JudgeCheck struct {
	Version            string                    `json:"schema_version"`
	Decision           string                    `json:"decision"`
	State              string                    `json:"state"`
	Reason             string                    `json:"reason"`
	Scope              string                    `json:"scope"`
	Authorization      string                    `json:"authorization"`
	MutationPermitted  bool                      `json:"mutation_permitted"`
	InputDigest        string                    `json:"input_sha256"`
	BindingDigest      string                    `json:"binding_sha256"`
	CatalogDigest      string                    `json:"catalog_sha256"`
	PolicyDigest       string                    `json:"policy_sha256"`
	Passed             int                       `json:"passed"`
	Total              int                       `json:"total"`
	MinimumBasisPoints int                       `json:"minimum_basis_points"`
	Evaluation         *gemara.ControlEvaluation `json:"native_evaluation"`
}

func judgeState(p JudgePolicy, o JudgeObservation, now time.Time) (gemara.Result, string) {
	if o.Version != "judge-observation/v1" || o.Authorization != "none" || o.MutationPermitted || o.Scorer != p.Scorer || o.SourceID != p.SourceID || o.ExperimentID != p.ExperimentID || o.Cohort != "experiment-scored-approvals" || !hash.MatchString(o.RecordsDigest) {
		return gemara.Unknown, "judge_source_mismatch"
	}
	start, e1 := time.Parse(time.RFC3339, o.WindowStart)
	end, e2 := time.Parse(time.RFC3339, o.WindowEnd)
	collected, e3 := time.Parse(time.RFC3339, o.CollectedAt)
	if e1 != nil || e2 != nil || e3 != nil || !end.Equal(collected) || end.Sub(start) != time.Duration(p.LookbackDays)*24*time.Hour || collected.After(now) || now.Sub(collected) > time.Duration(p.MaximumSnapshotAgeSeconds)*time.Second {
		return gemara.Unknown, "judge_snapshot_stale_or_invalid"
	}
	if !o.Complete || o.Invalid != 0 || o.ExcludedOtherSources < 0 || o.Total < 0 || o.Total > 1000000 || o.Passed < 0 || o.Passed > o.Total {
		return gemara.Unknown, "judge_population_incomplete_or_invalid"
	}
	if o.Total < p.MinimumSamples {
		return gemara.Unknown, "judge_sample_insufficient"
	}
	latest, err := time.Parse(time.RFC3339, o.LatestScoreAt)
	if err != nil || latest.Before(start) || latest.After(end) || now.Sub(latest) > time.Duration(p.MaximumScoreAgeSeconds)*time.Second {
		return gemara.Unknown, "judge_scores_stale_or_invalid"
	}
	if int64(o.Passed)*10000 < int64(o.Total)*int64(p.MinimumBasisPoints) {
		return gemara.Failed, "judge_approval_rate_below_threshold"
	}
	return gemara.Passed, "judge_approval_rate_satisfied"
}

// CheckJudge tests the observed judged-approval rate, not overall accuracy or
// an individual PR. Authentication and merge enforcement belong to the caller.
func CheckJudge(p JudgePolicy, o JudgeObservation, now time.Time) (JudgeCheck, error) {
	if p.Version != "judge-policy/v1" || p.Scorer != "review_correctly_approved" || p.SourceID == "" || len(p.SourceID) > 256 || p.ExperimentID == "" || len(p.ExperimentID) > 80 || p.LookbackDays < 1 || p.LookbackDays > 365 || p.MinimumSamples < 1 || p.MinimumSamples > 1000000 || p.MinimumBasisPoints < 1 || p.MinimumBasisPoints > 10000 || p.MaximumSnapshotAgeSeconds < 1 || p.MaximumSnapshotAgeSeconds > 3600 || p.MaximumScoreAgeSeconds < 1 || p.MaximumScoreAgeSeconds > 31*86400 || now.IsZero() {
		return JudgeCheck{}, fmt.Errorf("invalid_judge_policy")
	}
	var cat gemara.ControlCatalog
	var pol gemara.Policy
	if err := Decode(judgeCatalogJSON, &cat); err != nil {
		return JudgeCheck{}, err
	}
	if err := Decode(judgePolicyJSON, &pol); err != nil {
		return JudgeCheck{}, err
	}
	if len(cat.Controls) != 1 || len(pol.Adherence.AssessmentPlans) != 1 {
		return JudgeCheck{}, fmt.Errorf("invalid_judge_plan")
	}
	control := cat.Controls[0]
	if control.Id != "REVIEW-QUALITY" || len(control.AssessmentRequirements) != 1 {
		return JudgeCheck{}, fmt.Errorf("invalid_judge_plan")
	}
	req := control.AssessmentRequirements[0]
	plan := pol.Adherence.AssessmentPlans[0]
	if plan.RequirementId != req.Id || len(plan.EvaluationMethods) != 1 || plan.EvaluationMethods[0].Id != "evaluate-REVIEW-QUALITY" || !plan.EvaluationMethods[0].Required {
		return JudgeCheck{}, fmt.Errorf("invalid_judge_plan")
	}
	state, reason := judgeState(p, o, now)
	c := &gemara.ControlEvaluation{Name: control.Title, Control: gemara.EntryMapping{ReferenceId: cat.Metadata.Id, EntryId: control.Id}}
	a := c.AddAssessment(req.Id, req.Text, req.Applicability, []gemara.AssessmentStep{gemara.NamedStep(plan.EvaluationMethods[0].Id, func(any) (gemara.Result, string, gemara.ConfidenceLevel) { return state, reason, gemara.Undetermined })})
	a.Requirement.ReferenceId = cat.Metadata.Id
	a.Plan = &gemara.EntryMapping{ReferenceId: pol.Metadata.Id, EntryId: plan.Id}
	a.Run(&payload{})
	c.Result = a.Result
	c.Message = reason
	label, decision := "unknown", "hold"
	if a.Result == gemara.Passed {
		label, decision = "passed", "eligible"
	}
	if a.Result == gemara.Failed {
		label = "failed"
	}
	return JudgeCheck{Version: "judge-check/v1", Decision: decision, State: label, Reason: reason, Scope: "Experimental experiment-wide scored-approval rate; not a current-PR verdict or accuracy guarantee", Authorization: "none", InputDigest: Digest(o), BindingDigest: Digest(p), CatalogDigest: Digest(cat), PolicyDigest: Digest(pol), Passed: o.Passed, Total: o.Total, MinimumBasisPoints: p.MinimumBasisPoints, Evaluation: c}, nil
}
