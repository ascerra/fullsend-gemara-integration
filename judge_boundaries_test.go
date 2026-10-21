// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"testing"
	"time"
)

func TestJudgeMalformedInputsAndExactFreshnessBoundaries(t *testing.T) {
	cases := []struct {
		name   string
		change func(*JudgePolicy, *JudgeObservation, *time.Time)
		want   string
	}{
		{"window_length", func(_ *JudgePolicy, o *JudgeObservation, n *time.Time) {
			o.WindowStart = n.Add(-24 * time.Hour).Format(time.RFC3339)
		}, "unknown"},
		{"end_collected_differ", func(_ *JudgePolicy, o *JudgeObservation, n *time.Time) {
			o.WindowEnd = n.Add(-time.Second).Format(time.RFC3339)
		}, "unknown"},
		{"future_collected", func(_ *JudgePolicy, _ *JudgeObservation, n *time.Time) { *n = n.Add(-time.Second) }, "unknown"},
		{"bad_timestamp", func(_ *JudgePolicy, o *JudgeObservation, _ *time.Time) { o.WindowStart = "bad" }, "unknown"},
		{"before_window", func(_ *JudgePolicy, o *JudgeObservation, n *time.Time) {
			o.LatestScoreAt = n.Add(-366 * 24 * time.Hour).Format(time.RFC3339)
		}, "unknown"},
		{"bad_digest", func(_ *JudgePolicy, o *JudgeObservation, _ *time.Time) { o.RecordsDigest = "bad" }, "unknown"},
		{"authority", func(_ *JudgePolicy, o *JudgeObservation, _ *time.Time) { o.Authorization = "merge" }, "unknown"},
		{"schema", func(_ *JudgePolicy, o *JudgeObservation, _ *time.Time) { o.Version = "other" }, "unknown"},
		{"negative_passed", func(_ *JudgePolicy, o *JudgeObservation, _ *time.Time) { o.Passed = -1 }, "unknown"},
		{"negative_total", func(_ *JudgePolicy, o *JudgeObservation, _ *time.Time) { o.Total = -1 }, "unknown"},
		{"negative_excluded", func(_ *JudgePolicy, o *JudgeObservation, _ *time.Time) { o.ExcludedOtherSources = -1 }, "unknown"},
		{"snapshot_exact", func(_ *JudgePolicy, _ *JudgeObservation, n *time.Time) { *n = n.Add(900 * time.Second) }, "passed"},
		{"snapshot_old", func(_ *JudgePolicy, _ *JudgeObservation, n *time.Time) { *n = n.Add(901 * time.Second) }, "unknown"},
		{"score_exact", func(_ *JudgePolicy, o *JudgeObservation, n *time.Time) {
			o.LatestScoreAt = n.Add(-604800 * time.Second).Format(time.RFC3339)
		}, "passed"},
		{"score_old", func(_ *JudgePolicy, o *JudgeObservation, n *time.Time) {
			o.LatestScoreAt = n.Add(-604801 * time.Second).Format(time.RFC3339)
		}, "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, o, n := judgeExample()
			tc.change(&p, &o, &n)
			r, err := CheckJudge(p, o, n)
			if err != nil || r.State != tc.want {
				t.Fatalf("%s %v", r.State, err)
			}
		})
	}
}

func TestJudgePolicyBounds(t *testing.T) {
	changes := []func(*JudgePolicy){
		func(p *JudgePolicy) { p.Version = "bad" }, func(p *JudgePolicy) { p.Scorer = "other" }, func(p *JudgePolicy) { p.SourceID = "" }, func(p *JudgePolicy) { p.ExperimentID = "" },
		func(p *JudgePolicy) { p.LookbackDays = 0 }, func(p *JudgePolicy) { p.LookbackDays = 366 },
		func(p *JudgePolicy) { p.MinimumSamples = 0 }, func(p *JudgePolicy) { p.MinimumSamples = 1000001 },
		func(p *JudgePolicy) { p.MinimumBasisPoints = 10001 },
		func(p *JudgePolicy) { p.MaximumSnapshotAgeSeconds = 0 }, func(p *JudgePolicy) { p.MaximumSnapshotAgeSeconds = 3601 },
		func(p *JudgePolicy) { p.MaximumScoreAgeSeconds = 0 }, func(p *JudgePolicy) { p.MaximumScoreAgeSeconds = 31*86400 + 1 },
	}
	for _, change := range changes {
		p, o, n := judgeExample()
		change(&p)
		if _, err := CheckJudge(p, o, n); err == nil {
			t.Fatal("invalid policy accepted")
		}
	}
}

func TestCandidateHoldsUnknownQualityAndRejectsInvalidPolicy(t *testing.T) {
	b, e := baseline(t, "github", "clean")
	e.SourceKind = "collector-observation"
	p, o, n := judgeExample()
	e.CollectedAt = n.Format(time.RFC3339)
	for _, change := range []func(*JudgeObservation){func(o *JudgeObservation) { o.Complete = false }, func(o *JudgeObservation) { o.Total = 0; o.Passed = 0 }, func(o *JudgeObservation) { o.LatestScoreAt = n.Add(-8 * 24 * time.Hour).Format(time.RFC3339) }} {
		v := o
		change(&v)
		r, err := AssessCandidate(b, e, p, v, n)
		if err != nil || r.Candidate.Decision != "eligible" || r.Quality.State != "unknown" || r.Decision != "hold" {
			t.Fatalf("unknown failed to hold %v", err)
		}
	}
	p.MinimumBasisPoints = 0
	if _, err := AssessCandidate(b, e, p, o, n); err == nil {
		t.Fatal("invalid policy accepted")
	}
}
