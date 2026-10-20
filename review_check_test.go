// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"strings"
	"testing"

	gemara "github.com/gemaraproj/go-gemara"
)

func TestReadOnlyReviewCheck(t *testing.T) {
	head := strings.Repeat("a", 40)
	one, three := 1, 3
	base := ReviewProjection{Version: "review-projection/v1", Head: head, Authorization: "none", Review: Review{Completed: true, Action: "approve", Risk: &one, Findings: []Finding{}}}
	for _, tc := range []struct {
		name, decision, reason string
		change                 func(*ReviewProjection)
	}{
		{"complete", "eligible", "review_requirements_satisfied", func(*ReviewProjection) {}},
		{"missing-risk", "hold", "risk_assessment_missing", func(p *ReviewProjection) { p.Review.Risk = nil }},
		{"high-risk", "hold", "review_not_approved_low_risk", func(p *ReviewProjection) { p.Review.Risk = &three }},
		{"open-finding", "hold", "unresolved_finding", func(p *ReviewProjection) { p.Review.Findings = []Finding{{ID: "f1", Status: "open"}} }},
		{"human-dismissal", "hold", "unverified_finding_disposition", func(p *ReviewProjection) { p.Review.Findings = []Finding{{ID: "f1", Status: "dismissed_by_human"}} }},
		{"missing-findings", "hold", "review_incomplete", func(p *ReviewProjection) { p.Review.Findings = nil }},
		{"not-approved", "hold", "review_not_approved_low_risk", func(p *ReviewProjection) { p.Review.Action = "request-changes" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := base
			tc.change(&p)
			r, err := CheckReview(p, head, 2)
			if err != nil || r.Decision != tc.decision || r.Reason != tc.reason || r.Authorization != "none" || r.MutationPermitted || r.Evaluation == nil {
				t.Fatalf("unexpected check: %+v %v", r, err)
			}
			if tc.name == "missing-risk" && (r.State != "unknown" || r.Evaluation.Result != gemara.Unknown) {
				t.Fatal("missing risk was not native Gemara unknown")
			}
		})
	}
	for _, change := range []func(*ReviewProjection){func(p *ReviewProjection) { p.Head = strings.Repeat("b", 40) }, func(p *ReviewProjection) { p.Authorization = "merge" }, func(p *ReviewProjection) { p.MutationPermitted = true }} {
		p := base
		change(&p)
		if _, err := CheckReview(p, head, 2); err == nil {
			t.Fatal("invalid identity/authority accepted")
		}
	}
	a, _ := CheckReview(base, head, 2)
	b, _ := CheckReview(base, head, 3)
	if a.InputDigest == b.InputDigest {
		t.Fatal("risk threshold omitted from input digest")
	}
}
