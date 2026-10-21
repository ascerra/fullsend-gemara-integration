// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"strings"
	"testing"
	"time"
)

func judgeExample() (JudgePolicy, JudgeObservation, time.Time) {
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	p := JudgePolicy{Version: "judge-policy/v1", Scorer: "review_correctly_approved", SourceID: "synthetic.judge", ExperimentID: "1", LookbackDays: 365, MinimumSamples: 100, MinimumBasisPoints: 9900, MaximumSnapshotAgeSeconds: 900, MaximumScoreAgeSeconds: 7 * 86400}
	o := JudgeObservation{Version: "judge-observation/v1", Scorer: p.Scorer, SourceID: p.SourceID, ExperimentID: p.ExperimentID, Cohort: "experiment-scored-approvals", WindowStart: now.Add(-365 * 24 * time.Hour).Format(time.RFC3339), WindowEnd: now.Format(time.RFC3339), CollectedAt: now.Format(time.RFC3339), LatestScoreAt: now.Add(-time.Hour).Format(time.RFC3339), Passed: 99, Total: 100, Complete: true, RecordsDigest: strings.Repeat("a", 64), Authorization: "none"}
	return p, o, now
}

func TestJudgeFailClosedAndExactThreshold(t *testing.T) {
	cases := []struct {
		name, reason, state string
		change              func(*JudgePolicy, *JudgeObservation)
	}{
		{"exact99", "judge_approval_rate_satisfied", "passed", func(*JudgePolicy, *JudgeObservation) {}},
		{"below99", "judge_approval_rate_below_threshold", "failed", func(_ *JudgePolicy, o *JudgeObservation) { o.Passed = 98 }},
		{"live_shape", "judge_approval_rate_below_threshold", "failed", func(_ *JudgePolicy, o *JudgeObservation) { o.Passed = 238; o.Total = 254 }},
		{"rounded99_not_pass", "judge_approval_rate_below_threshold", "failed", func(_ *JudgePolicy, o *JudgeObservation) { o.Passed = 989; o.Total = 1000 }},
		{"few", "judge_sample_insufficient", "unknown", func(_ *JudgePolicy, o *JudgeObservation) { o.Passed = 1; o.Total = 1 }},
		{"none", "judge_sample_insufficient", "unknown", func(_ *JudgePolicy, o *JudgeObservation) { o.Passed = 0; o.Total = 0 }},
		{"truncated", "judge_population_incomplete_or_invalid", "unknown", func(_ *JudgePolicy, o *JudgeObservation) { o.Complete = false }},
		{"malformed_score", "judge_population_incomplete_or_invalid", "unknown", func(_ *JudgePolicy, o *JudgeObservation) { o.Invalid = 1 }},
		{"wrong_source", "judge_source_mismatch", "unknown", func(_ *JudgePolicy, o *JudgeObservation) { o.SourceID = "other" }},
		{"wrong_experiment", "judge_source_mismatch", "unknown", func(_ *JudgePolicy, o *JudgeObservation) { o.ExperimentID = "2" }},
		{"wrong_scorer", "judge_source_mismatch", "unknown", func(_ *JudgePolicy, o *JudgeObservation) { o.Scorer = "cost" }},
		{"wrong_cohort", "judge_source_mismatch", "unknown", func(_ *JudgePolicy, o *JudgeObservation) { o.Cohort = "all-reviews" }},
		{"stale_snapshot", "judge_snapshot_stale_or_invalid", "unknown", func(_ *JudgePolicy, o *JudgeObservation) {
			d, _ := time.Parse(time.RFC3339, o.CollectedAt)
			o.CollectedAt = d.Add(-time.Hour).Format(time.RFC3339)
		}},
		{"stale_score", "judge_scores_stale_or_invalid", "unknown", func(_ *JudgePolicy, o *JudgeObservation) {
			d, _ := time.Parse(time.RFC3339, o.CollectedAt)
			o.LatestScoreAt = d.Add(-8 * 24 * time.Hour).Format(time.RFC3339)
		}},
		{"future_score", "judge_scores_stale_or_invalid", "unknown", func(_ *JudgePolicy, o *JudgeObservation) {
			d, _ := time.Parse(time.RFC3339, o.CollectedAt)
			o.LatestScoreAt = d.Add(time.Hour).Format(time.RFC3339)
		}},
		{"bad_count", "judge_population_incomplete_or_invalid", "unknown", func(_ *JudgePolicy, o *JudgeObservation) { o.Passed = 101 }},
		{"privilege", "judge_source_mismatch", "unknown", func(_ *JudgePolicy, o *JudgeObservation) { o.MutationPermitted = true }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, o, now := judgeExample()
			tc.change(&p, &o)
			r, err := CheckJudge(p, o, now)
			if err != nil {
				t.Fatal(err)
			}
			if r.State != tc.state || r.Reason != tc.reason {
				t.Fatalf("got %s %s", r.State, r.Reason)
			}
			if (r.Decision == "eligible") != (tc.state == "passed") || r.MutationPermitted || r.Authorization != "none" || r.Evaluation == nil {
				t.Fatal("unsafe assessment")
			}
		})
	}
}

func TestJudgeRejectsInvalidPolicy(t *testing.T) {
	p, o, now := judgeExample()
	p.MinimumBasisPoints = 0
	if _, err := CheckJudge(p, o, now); err == nil {
		t.Fatal("accepted disabled threshold")
	}
}
