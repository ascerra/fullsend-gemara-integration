// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"encoding/json"
	"fmt"
	"strings"
)

// DemoCaseNames are scenarios, not a second set of policy rules. Only the two
// forge baselines are stored as JSON; variations below keep the demo inspectable.
var DemoCaseNames = []string{"clean", "one-line-governance", "larger-allowed", "open-finding", "stale-head", "failed-check", "missing-check", "wrong-project", "expired-grant"}

func FixtureCase(binding Binding, evidence Evidence, name string) (Binding, Evidence, error) {
	var b Binding
	var e Evidence
	raw, _ := json.Marshal(binding)
	_ = json.Unmarshal(raw, &b)
	raw, _ = json.Marshal(evidence)
	_ = json.Unmarshal(raw, &e)
	switch name {
	case "clean":
	case "one-line-governance":
		p := ".github/workflows/ci.yml"
		if b.Identity.Forge == "gitlab" {
			p = ".gitlab-ci.yml"
		}
		e.Files = []ChangedFile{{Path: p, Kind: "modified", Mode: "100644"}}
	case "larger-allowed":
		e.Files = nil
		for i := 0; i < 120; i++ {
			e.Files = append(e.Files, ChangedFile{Path: fmt.Sprintf("docs/reference/section-%d.md", i), Kind: "modified", Mode: "100644"})
		}
	case "open-finding":
		e.Review.Findings = []Finding{{ID: "f_example", Status: "open"}}
	case "stale-head":
		e.Subject.Head = strings.Repeat("d", 40)
	case "failed-check":
		if len(e.Checks) == 0 {
			return Binding{}, Evidence{}, fmt.Errorf("demo_baseline_check_missing")
		}
		e.Checks[0].Status = "failure"
	case "missing-check":
		e.Checks = nil
	case "wrong-project":
		e.Subject.Identity.ProjectID = "99999"
		e.Publication.Subject.Identity.ProjectID = "99999"
	case "expired-grant":
		b.GrantExpiresAt = "2026-10-18T18:00:00Z"
		e.Subject.BindingDigest = Digest(b)
		e.Publication.Subject = e.Subject
	default:
		return Binding{}, Evidence{}, fmt.Errorf("unknown_demo_case")
	}
	return b, e, nil
}
