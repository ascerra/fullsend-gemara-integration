// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestProjectReview(t *testing.T) {
	head := strings.Repeat("a", 40)
	result := fmt.Sprintf(`{"action":"approve","head_sha":%q,"risk_assessment":{"score":1,"level":"low"},"body":"ignored model text"}`, head)
	ledger := `{"version":2,"action":"approve","findings":[{"id":"f1","severity":"low","category":"bug","file":null},{"id":"f2","severity":"high","category":"security","file":"code.go"}],"dispositions":[{"id":"f2","status":"resolved_by_change"}]}`
	r, err := ProjectReview([]byte(result), []byte(ledger), head)
	if err != nil || !r.Completed || r.Action != "approve" || r.Risk == nil || *r.Risk != 1 || len(r.Findings) != 2 || r.Findings[0].Status != "open" || r.Findings[1].Status != "resolved_by_change" {
		t.Fatalf("projection lost review information: %+v %v", r, err)
	}
	for _, tc := range []struct{ name, result, ledger, head string }{
		{"stale-head", result, ledger, strings.Repeat("b", 40)},
		{"conflicting-action", strings.Replace(result, `"action":"approve"`, `"action":"request-changes"`, 1), ledger, head},
		{"missing-ledger", result, `{}`, head},
		{"null-findings", result, `{"version":2,"action":"approve","findings":null}`, head},
		{"duplicate-keys", result, `{"version":2,"version":2,"action":"approve","findings":[]}`, head},
		{"duplicate-finding", result, strings.Replace(ledger, `"id":"f2","severity"`, `"id":"f1","severity"`, 1), head},
		{"orphan-disposition", result, strings.Replace(ledger, `"id":"f2","status"`, `"id":"missing","status"`, 1), head},
		{"unknown-ledger-field", result, strings.Replace(ledger, `"version":2`, `"version":2,"override":true`, 1), head},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ProjectReview([]byte(tc.result), []byte(tc.ledger), tc.head); err == nil {
				t.Fatal("invalid publication accepted")
			}
		})
	}
	for _, risk := range []string{`null`, `{}`, `{"score":1,"level":"low","degraded":false}`, `{"score":1,"level":"high"}`, `{"score":0,"level":"low"}`, `{"score":1.5,"level":"low"}`} {
		r, err := ProjectReview([]byte(fmt.Sprintf(`{"action":"approve","head_sha":%q,"risk_assessment":%s}`, head, risk)), []byte(ledger), head)
		if err != nil || r.Risk != nil || len(r.Findings) != 2 {
			t.Fatalf("unknown risk was invented or findings lost: %+v %v", r, err)
		}
	}
	r, err = ProjectReview([]byte(result), []byte(`{"version":2,"action":"approve","findings":[]}`), head)
	if err != nil || r.Findings == nil || len(r.Findings) != 0 {
		t.Fatal("explicit empty ledger should remain empty")
	}
}

func TestReviewProjectionPolicyParity(t *testing.T) {
	now, _ := time.Parse(time.RFC3339, "2026-10-19T18:00:00Z")
	for _, forge := range []string{"github", "gitlab"} {
		t.Run(forge, func(t *testing.T) {
			raw, err := os.ReadFile("examples/" + forge + "/binding.json")
			if err != nil {
				t.Fatal(err)
			}
			var b Binding
			if err := Decode(raw, &b); err != nil {
				t.Fatal(err)
			}
			raw, err = os.ReadFile("testdata/" + forge + "/clean.json")
			if err != nil {
				t.Fatal(err)
			}
			e, err := NormalizeFixture(raw)
			if err != nil {
				t.Fatal(err)
			}
			result := []byte(fmt.Sprintf(`{"action":"approve","head_sha":%q,"risk_assessment":{"score":1,"level":"low"}}`, e.Subject.Head))
			for _, tc := range []struct{ ledger, decision string }{
				{`{"version":2,"action":"approve","findings":[]}`, "eligible"},
				{`{"version":2,"action":"approve","findings":[{"id":"f1","severity":"low","category":"bug","file":null}]}`, "hold"},
				{`{"version":2,"action":"approve","findings":[{"id":"f1","severity":"low","category":"bug","file":null}],"dispositions":[{"id":"f1","status":"dismissed_by_human"}]}`, "hold"},
			} {
				e.Review, err = ProjectReview(result, []byte(tc.ledger), e.Subject.Head)
				if err != nil {
					t.Fatal(err)
				}
				r, err := EvaluateFixture(b, e, now)
				if err != nil || r.Decision != tc.decision || r.MutationPermitted || r.Authorization != "none" {
					t.Fatalf("unexpected policy result: %+v %v", r, err)
				}
			}
		})
	}
}
