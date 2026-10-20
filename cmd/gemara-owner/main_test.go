// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func invoke(t *testing.T, args ...string) (string, error) {
	t.Helper()
	savedArgs, savedOut := os.Args, os.Stdout
	f, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	os.Args = append([]string{"gemara-owner"}, args...)
	os.Stdout = f
	defer func() { os.Args = savedArgs; os.Stdout = savedOut; f.Close() }()
	err = run()
	if _, seekErr := f.Seek(0, 0); seekErr != nil {
		t.Fatal(seekErr)
	}
	raw, readErr := os.ReadFile(f.Name())
	if readErr != nil {
		t.Fatal(readErr)
	}
	return string(raw), err
}

func TestCLIProducesNativeFixtureReport(t *testing.T) {
	out, err := invoke(t, "evaluate-fixture", "--binding", "../../examples/gitlab/binding.json", "--fixture", "../../testdata/gitlab/clean.json", "--at", "2026-10-19T18:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	var r map[string]any
	if json.Unmarshal([]byte(out), &r) != nil || r["decision"] != "eligible" || r["mutation_permitted"] != false || r["authorization"] != "none" || r["evaluation_log"] == nil {
		t.Fatal("incorrect report or authority")
	}
}

func TestHeldCLIExitsWithFailure(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/github/clean.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture map[string]any
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	observation := fixture["observation"].(map[string]any)
	observation["checks"] = []any{}
	raw, _ = json.Marshal(fixture)
	p := filepath.Join(t.TempDir(), "held.json")
	if err := os.WriteFile(p, raw, 0600); err != nil {
		t.Fatal(err)
	}
	out, err := invoke(t, "evaluate-fixture", "--binding", "../../examples/github/binding.json", "--fixture", p, "--at", "2026-10-19T18:00:00Z")
	if err == nil || err.Error() != "policy_held" || !strings.Contains(out, `"decision":"hold"`) {
		t.Fatal("hold did not fail the CLI")
	}
}

func TestDemoRunsAllPairedCases(t *testing.T) {
	out, err := invoke(t, "demo", "--root", "../..", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Forge  string `json:"forge"`
		Case   string `json:"case"`
		Report struct {
			Decision          string `json:"decision"`
			Authorization     string `json:"authorization"`
			MutationPermitted bool   `json:"mutation_permitted"`
		} `json:"report"`
	}
	if json.Unmarshal([]byte(out), &rows) != nil || len(rows) != 20 {
		t.Fatal("incomplete demo")
	}
	stale := map[string]bool{}
	for _, row := range rows {
		if row.Case == "stale-evidence" {
			if row.Report.Decision != "hold" || row.Report.Authorization != "none" || row.Report.MutationPermitted {
				t.Fatalf("stale evidence did not hold without authority on %s", row.Forge)
			}
			stale[row.Forge] = true
		}
	}
	if !stale["github"] || !stale["gitlab"] {
		t.Fatal("stale-evidence case missing for a forge")
	}
}

func TestCLIRejectsMissingTimeAndUnknownCommands(t *testing.T) {
	for _, args := range [][]string{{"evaluate-fixture"}, {"project-review"}, {"merge"}, {"demo", "unexpected"}} {
		if _, err := invoke(t, args...); err == nil {
			t.Fatal("invalid command succeeded")
		}
	}
}

func TestCLIReviewProjectionHasNoAuthority(t *testing.T) {
	dir := t.TempDir()
	head := strings.Repeat("a", 40)
	result, ledger := filepath.Join(dir, "result.json"), filepath.Join(dir, "ledger.json")
	if err := os.WriteFile(result, []byte(`{"action":"approve","head_sha":"`+head+`","risk_assessment":{"score":1,"level":"low"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ledger, []byte(`{"version":2,"action":"approve","findings":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := invoke(t, "project-review", "--result", result, "--ledger", ledger, "--head", head)
	if err != nil {
		t.Fatal(err)
	}
	var r map[string]any
	if json.Unmarshal([]byte(out), &r) != nil || r["schema_version"] != "review-projection/v1" || r["head_sha"] != head || r["authorization"] != "none" || r["mutation_permitted"] != false {
		t.Fatal("projection claimed authority or lost its identity")
	}
	if _, err := invoke(t, "project-review", "--result", result, "--ledger", ledger, "--head", strings.Repeat("b", 40)); err == nil {
		t.Fatal("stale head accepted")
	}
}

func TestCLIReviewCheckHoldsMissingRisk(t *testing.T) {
	head := strings.Repeat("a", 40)
	path := filepath.Join(t.TempDir(), "projection.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":"review-projection/v1","head_sha":"`+head+`","review":{"completed":true,"action":"approve","risk_score":null,"findings":[]},"authorization":"none","mutation_permitted":false}`), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := invoke(t, "check-review", "--projection", path, "--head", head, "--maximum-risk", "2")
	if err == nil || err.Error() != "policy_held" || !strings.Contains(out, `"risk_assessment_missing"`) || !strings.Contains(out, `"native_evaluation"`) || !strings.Contains(out, `"mutation_permitted":false`) {
		t.Fatalf("missing risk failed to hold: %s %v", out, err)
	}
}
