// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	gemara "github.com/gemaraproj/go-gemara"
)

func baseline(t *testing.T, forge, scenario string) (Binding, Evidence) {
	t.Helper()
	binding := "binding.json"
	raw, err := os.ReadFile(filepath.Join("examples", forge, binding))
	if err != nil {
		t.Fatal(err)
	}
	var b Binding
	if err := Decode(raw, &b); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(filepath.Join("testdata", forge, "clean.json"))
	if err != nil {
		t.Fatal(err)
	}
	e, err := NormalizeFixture(raw)
	if err != nil {
		t.Fatal(err)
	}
	b, e, err = FixtureCase(b, e, scenario)
	if err != nil {
		t.Fatal(err)
	}
	return b, e
}

func at() time.Time { n, _ := time.Parse(time.RFC3339, "2026-10-19T18:00:00Z"); return n }

func evaluateForTest(t *testing.T, b Binding, e Evidence) Report {
	t.Helper()
	r, err := EvaluateFixture(b, e, at())
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestPairedForgeParityAndNativeLogs(t *testing.T) {
	cases := map[string]string{"clean": "eligible", "larger-allowed": "eligible", "one-line-governance": "hold", "open-finding": "hold", "stale-head": "hold", "failed-check": "hold", "missing-check": "hold", "wrong-project": "hold", "expired-grant": "hold"}
	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			var reference Report
			for _, forge := range []string{"github", "gitlab"} {
				b, e := baseline(t, forge, name)
				r := evaluateForTest(t, b, e)
				if r.Decision != want || r.Engine != "gemara" || r.MutationPermitted || r.Authorization != "none" || r.EvidenceKind != "synthetic-fixture" {
					t.Fatalf("incorrect outcome or authority: %+v", r)
				}
				if len(r.Assessments) != 5 || len(r.Log.Evaluations) != 5 {
					t.Fatal("missing assessments")
				}
				if (r.Log.Result == gemara.Passed) != (want == "eligible") {
					t.Fatalf("native aggregate disagrees: %s", r.Log.Result)
				}
				for i, c := range r.Log.Evaluations {
					if len(c.AssessmentLogs) != 1 {
						t.Fatal("missing native assessment")
					}
					a := c.AssessmentLogs[0]
					if a.Plan == nil || a.Requirement.ReferenceId != "owner-merge-controls" || a.Requirement.EntryId != r.Assessments[i].ID+"-1" || len(a.Evidence) == 0 {
						t.Fatal("native policy/requirement/evidence trace missing")
					}
				}
				if forge == "github" {
					reference = r
				} else if r.PolicyDigest != reference.PolicyDigest || r.CatalogDigest != reference.CatalogDigest || !reflect.DeepEqual(r.Assessments, reference.Assessments) {
					t.Fatal("shared policy or assessment outcome diverged by forge")
				}
			}
		})
	}
}

func TestGuardConditionsHold(t *testing.T) {
	tests := map[string]func(*Binding, *Evidence){
		"untrusted-host":       func(b *Binding, e *Evidence) { e.Subject.Identity.Host = "attacker.example" },
		"different-policy":     func(b *Binding, e *Evidence) { e.Subject.PolicyDigest = strings.Repeat("0", 64) },
		"changed-owner-config": func(b *Binding, e *Evidence) { b.MaximumRisk = 2 },
		"stale":                func(b *Binding, e *Evidence) { e.CollectedAt = "2026-10-19T17:00:00Z" },
		"future":               func(b *Binding, e *Evidence) { e.CollectedAt = "2026-10-19T19:00:00Z" },
		"disabled-grant": func(b *Binding, e *Evidence) {
			b.GrantEnabled = false
			e.Subject.BindingDigest = Digest(*b)
			e.Publication.Subject = e.Subject
		},
		"draft":           func(b *Binding, e *Evidence) { e.Draft = true },
		"closed":          func(b *Binding, e *Evidence) { e.Open = false },
		"missing-files":   func(b *Binding, e *Evidence) { e.Files = nil },
		"duplicate-files": func(b *Binding, e *Evidence) { e.Files = append(e.Files, e.Files[0]) },
		"rename":          func(b *Binding, e *Evidence) { e.Files[0].PreviousPath = ".github/workflows/ci.yml" },
		"symlink":         func(b *Binding, e *Evidence) { e.Files[0].Mode = "120000" },
		"traversal":       func(b *Binding, e *Evidence) { e.Files[0].Path = "docs/../.gitlab-ci.yml" },
		"backslash":       func(b *Binding, e *Evidence) { e.Files[0].Path = "docs\\policy.md" },
		"governed-overrides-allowed": func(b *Binding, e *Evidence) {
			b.AllowedPaths = append(b.AllowedPaths, ".github/**")
			e.Subject.BindingDigest = Digest(*b)
			e.Publication.Subject = e.Subject
			e.Files[0].Path = ".github/workflows/ci.yml"
		},
		"risk-missing":           func(b *Binding, e *Evidence) { e.Review.Risk = nil },
		"high-risk":              func(b *Binding, e *Evidence) { n := 4; e.Review.Risk = &n },
		"unapproved":             func(b *Binding, e *Evidence) { e.Review.Action = "request_changes" },
		"review-running":         func(b *Binding, e *Evidence) { e.Review.Completed = false },
		"missing-finding-ledger": func(b *Binding, e *Evidence) { e.Review.Findings = nil },
		"unverified-dismissal":   func(b *Binding, e *Evidence) { e.Review.Findings = []Finding{{"f_a", "dismissed_by_human"}} },
		"reclassified-finding":   func(b *Binding, e *Evidence) { e.Review.Findings = []Finding{{"f_a", "reclassified"}} },
		"duplicate-finding": func(b *Binding, e *Evidence) {
			e.Review.Findings = []Finding{{"f_a", "resolved_by_change"}, {"f_a", "resolved_by_change"}}
		},
		"check-skipped":            func(b *Binding, e *Evidence) { e.Checks[0].Status = "skipped" },
		"duplicate-required-check": func(b *Binding, e *Evidence) { e.Checks = append(e.Checks, e.Checks[0]) },
		"check-old-head":           func(b *Binding, e *Evidence) { e.Checks[0].Head = strings.Repeat("f", 40) },
		"wrong-workflow":           func(b *Binding, e *Evidence) { e.Publication.Workflow = "untrusted-workflow" },
		"missing-run":              func(b *Binding, e *Evidence) { e.Publication.Run = "" },
		"bad-attempt":              func(b *Binding, e *Evidence) { e.Publication.Attempt = 0 },
		"bad-artifact-digest":      func(b *Binding, e *Evidence) { e.Publication.ArtifactDigest = "unknown" },
		"wrong-target":             func(b *Binding, e *Evidence) { e.Publication.Subject.Base = strings.Repeat("e", 40) },
		"unprotected":              func(b *Binding, e *Evidence) { e.Protections.ProtectedTarget = false },
		"optional-policy":          func(b *Binding, e *Evidence) { e.Protections.RequiredPolicyGate = false },
		"open-discussion":          func(b *Binding, e *Evidence) { e.Protections.DiscussionsResolved = false },
		"bypass":                   func(b *Binding, e *Evidence) { e.Protections.BypassEnabled = true },
		"direct-merge":             func(b *Binding, e *Evidence) { e.Protections.Mechanism = "direct-push" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			b, e := baseline(t, "github", "clean")
			mutate(&b, &e)
			r := evaluateForTest(t, b, e)
			if r.Decision != "hold" || r.MutationPermitted {
				t.Fatal("unsafe condition became eligible")
			}
		})
	}
}

func TestSizeIsNotAControl(t *testing.T) {
	b, e := baseline(t, "github", "larger-allowed")
	if len(e.Files) != 120 {
		t.Fatal("large fixture missing")
	}
	if evaluateForTest(t, b, e).Decision != "eligible" {
		t.Fatal("allowed scope rejected because of size")
	}
	b, e = baseline(t, "github", "one-line-governance")
	if len(e.Files) != 1 || evaluateForTest(t, b, e).Decision != "hold" {
		t.Fatal("single governed file allowed")
	}
	raw, _ := json.Marshal(b)
	if strings.Contains(string(raw), "lines") || strings.Contains(string(raw), "size") {
		t.Fatal("size entered owner configuration")
	}
}

func TestFixtureCannotClaimLiveAuthority(t *testing.T) {
	b, e := baseline(t, "github", "clean")
	e.SourceKind = "authenticated-live"
	if _, err := EvaluateFixture(b, e, at()); err == nil {
		t.Fatal("live assertion accepted")
	}
	raw, err := os.ReadFile("testdata/github/clean.json")
	if err != nil {
		t.Fatal(err)
	}
	raw = []byte(strings.ReplaceAll(string(raw), "synthetic-fixture", "authenticated-live"))
	if _, err := NormalizeFixture(raw); err == nil {
		t.Fatal("fixture promoted itself")
	}
}

func TestStrictDecodingAndConflictingSubjects(t *testing.T) {
	for _, s := range []string{`{"a":1,"a":2}`, `{"a":{}} {}`, `{"secret_token":"not-a-real-secret"}`, `{"a":[]}`} {
		var b Binding
		if Decode([]byte(s), &b) == nil {
			t.Fatalf("bad input accepted: %s", s)
		}
	}
	var x any
	if Decode([]byte(`{"x":{"a":1,"a":2}}`), &x) == nil {
		t.Fatal("nested duplicate accepted")
	}
	if Decode([]byte(strings.Repeat(" ", 1024*1024+1)), &x) == nil {
		t.Fatal("oversized input accepted")
	}
	raw, _ := os.ReadFile("testdata/github/clean.json")
	var f FixtureEnvelope
	if err := Decode(raw, &f); err != nil {
		t.Fatal(err)
	}
	f.GitHub.PullRequest.Number++
	raw, _ = json.Marshal(f)
	if _, err := NormalizeFixture(raw); err == nil {
		t.Fatal("conflicting change numbers accepted")
	}
	f.GitLab = &GitLabFixture{}
	raw, _ = json.Marshal(f)
	if _, err := NormalizeFixture(raw); err == nil {
		t.Fatal("mixed forges accepted")
	}
}

func TestInvalidBindingsFail(t *testing.T) {
	for name, mutate := range map[string]func(*Binding){"empty-host": func(b *Binding) { b.Identity.Host = "" }, "host-url": func(b *Binding) { b.Identity.Host = "https://github.com" }, "wildcard-path": func(b *Binding) { b.AllowedPaths = []string{"**"} }, "traversal": func(b *Binding) { b.AllowedPaths = []string{"../docs/**"} }, "empty-checks": func(b *Binding) { b.RequiredChecks = nil }, "duplicate-checks": func(b *Binding) { b.RequiredChecks = append(b.RequiredChecks, b.RequiredChecks[0]) }, "unbounded-age": func(b *Binding) { b.MaximumAgeSeconds = 0 }} {
		t.Run(name, func(t *testing.T) {
			b, e := baseline(t, "github", "clean")
			mutate(&b)
			if _, err := EvaluateFixture(b, e, at()); err == nil {
				t.Fatal("invalid owner configuration accepted")
			}
		})
	}
}
