// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"fmt"
	gemara "github.com/gemaraproj/go-gemara"
)

// ReviewProjection is bounded data produced after official Review publication.
// It contains no proof of forge identity, freshness or merge authorization.
type ReviewProjection struct {
	Version           string `json:"schema_version"`
	Head              string `json:"head_sha"`
	Review            Review `json:"review"`
	Authorization     string `json:"authorization"`
	MutationPermitted bool   `json:"mutation_permitted"`
}

type ReviewCheck struct {
	Version           string                    `json:"schema_version"`
	Head              string                    `json:"head_sha"`
	EvidenceKind      string                    `json:"evidence_kind"`
	Scope             string                    `json:"scope"`
	Decision          string                    `json:"decision"`
	State             string                    `json:"state"`
	Reason            string                    `json:"reason"`
	Authorization     string                    `json:"authorization"`
	MutationPermitted bool                      `json:"mutation_permitted"`
	InputDigest       string                    `json:"input_sha256"`
	CatalogDigest     string                    `json:"catalog_sha256"`
	PolicyDigest      string                    `json:"policy_sha256"`
	MaximumRisk       int                       `json:"maximum_risk"`
	Evaluation        *gemara.ControlEvaluation `json:"native_evaluation"`
}

// CheckReview applies the common REVIEW-CLEAR requirement to observed data.
// Its caller must authenticate and bind the producer before supplying data.
// This single-control result cannot authorize any forge mutation.
func CheckReview(p ReviewProjection, expectedHead string, maximumRisk int) (ReviewCheck, error) {
	var r ReviewCheck
	if p.Version != "review-projection/v1" || !sha.MatchString(expectedHead) || p.Head != expectedHead || p.Authorization != "none" || p.MutationPermitted || maximumRisk < 1 || maximumRisk > 5 {
		return r, fmt.Errorf("invalid_review_check_input")
	}
	var cat gemara.ControlCatalog
	var pol gemara.Policy
	if err := Decode(catalogJSON, &cat); err != nil {
		return r, err
	}
	if err := Decode(policyJSON, &pol); err != nil {
		return r, err
	}
	for _, control := range cat.Controls {
		if control.Id != "REVIEW-CLEAR" {
			continue
		}
		if len(control.AssessmentRequirements) != 1 {
			return r, fmt.Errorf("invalid_review_requirement")
		}
		req := control.AssessmentRequirements[0]
		for _, plan := range pol.Adherence.AssessmentPlans {
			if plan.RequirementId != req.Id {
				continue
			}
			if len(plan.EvaluationMethods) != 1 || plan.EvaluationMethods[0].Id != "evaluate-REVIEW-CLEAR" || !plan.EvaluationMethods[0].Required {
				return r, fmt.Errorf("invalid_review_plan")
			}
			state, reason := checkReview(p.Review, maximumRisk)
			c := &gemara.ControlEvaluation{Name: control.Title, Control: gemara.EntryMapping{ReferenceId: cat.Metadata.Id, EntryId: control.Id}}
			a := c.AddAssessment(req.Id, req.Text, req.Applicability, []gemara.AssessmentStep{gemara.NamedStep(plan.EvaluationMethods[0].Id, func(any) (gemara.Result, string, gemara.ConfidenceLevel) { return state, reason, gemara.Undetermined })})
			a.Requirement.ReferenceId = cat.Metadata.Id
			a.Plan = &gemara.EntryMapping{ReferenceId: pol.Metadata.Id, EntryId: plan.Id}
			a.Run(&payload{})
			c.Result = a.Result
			c.Message = reason
			label := "unknown"
			decision := "hold"
			if a.Result == gemara.Passed {
				label = "passed"
				decision = "eligible"
			}
			if a.Result == gemara.Failed {
				label = "failed"
			}
			catalog, policy := SharedDigests()
			return ReviewCheck{Version: "review-check/v1", Head: expectedHead, EvidenceKind: "data-only-review-observation", Scope: "REVIEW-CLEAR only; forge authentication and other controls are separate", Decision: decision, State: label, Reason: reason, Authorization: "none", InputDigest: Digest(struct {
				Projection  ReviewProjection
				MaximumRisk int
			}{p, maximumRisk}), CatalogDigest: catalog, PolicyDigest: policy, MaximumRisk: maximumRisk, Evaluation: c}, nil
		}
	}
	return r, fmt.Errorf("missing_review_plan")
}

func checkReview(review Review, maximumRisk int) (gemara.Result, string) {
	if review.Risk == nil {
		return gemara.Unknown, "risk_assessment_missing"
	}
	if !review.Completed || *review.Risk < 1 || *review.Risk > 5 || review.Findings == nil {
		return gemara.Unknown, "review_incomplete"
	}
	if review.Action != "approve" || *review.Risk > maximumRisk {
		return gemara.Failed, "review_not_approved_low_risk"
	}
	seen := map[string]bool{}
	for _, f := range review.Findings {
		if f.ID == "" || seen[f.ID] {
			return gemara.Unknown, "finding_invalid"
		}
		seen[f.ID] = true
		// Human dismissals need authenticated disposition proof, which this
		// initial fixture contract deliberately does not pretend to provide.
		switch f.Status {
		case "open", "reclassified":
			return gemara.Failed, "unresolved_finding"
		case "resolved_by_change":
		default:
			return gemara.Unknown, "unverified_finding_disposition"
		}
	}
	return gemara.Passed, "review_requirements_satisfied"
}
