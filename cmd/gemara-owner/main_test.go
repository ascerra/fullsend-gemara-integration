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
	var rows []json.RawMessage
	if json.Unmarshal([]byte(out), &rows) != nil || len(rows) != 18 {
		t.Fatal("incomplete demo")
	}
}

func TestCLIRejectsMissingTimeAndUnknownCommands(t *testing.T) {
	for _, args := range [][]string{{"evaluate-fixture"}, {"merge"}, {"demo", "unexpected"}} {
		if _, err := invoke(t, args...); err == nil {
			t.Fatal("invalid command succeeded")
		}
	}
}
