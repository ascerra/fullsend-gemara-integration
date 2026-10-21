// SPDX-License-Identifier: Apache-2.0
package integration

import (
	"testing"
	"time"
)

func TestCandidateRequiresEveryControlAndJudge(t *testing.T) {
	for _, forge := range []string{"github", "gitlab"} {
		for _, scenario := range []string{"clean", "open-finding", "stale-head", "failed-check", "missing-check", "wrong-project", "expired-grant", "one-line-governance"} {
			for _, qualityPassed := range []bool{true, false} {
				t.Run(forge+"/"+scenario+"/"+map[bool]string{true: "good-quality", false: "bad-quality"}[qualityPassed], func(t *testing.T) {
					b, e := baseline(t, forge, scenario)
					e.SourceKind = "collector-observation"
					p, o, _ := judgeExample()
					now := at()
					o.WindowEnd = now.Format(time.RFC3339)
					o.CollectedAt = o.WindowEnd
					o.WindowStart = now.Add(-365 * 24 * time.Hour).Format(time.RFC3339)
					o.LatestScoreAt = now.Add(-time.Hour).Format(time.RFC3339)
					if !qualityPassed {
						o.Passed = 94
					}
					r, err := AssessCandidate(b, e, p, o, now)
					if err != nil {
						t.Fatal(err)
					}
					want := scenario == "clean" && qualityPassed
					if (r.Decision == "eligible") != want || r.Authorization != "none" || r.MutationPermitted || len(r.Candidate.Assessments) != 5 || r.Quality.Evaluation == nil {
						t.Fatalf("unexpected result %s", r.Decision)
					}
				})
			}
		}
	}
}
func TestCandidateDoesNotAcceptFixtureAsCollectorEvidence(t *testing.T) {
	b, e := baseline(t, "github", "clean")
	if _, err := EvaluateCandidate(b, e, at()); err == nil {
		t.Fatal("fixture promoted")
	}
	e.SourceKind = "collector-observation"
	if _, err := EvaluateFixture(b, e, at()); err == nil {
		t.Fatal("fixture evaluator accepted live marker")
	}
}
