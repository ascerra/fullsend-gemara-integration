// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"encoding/json"
	"fmt"
)

// ProjectReview converts the official post-script's bounded result and v2
// findings ledger after successful publication. The caller must authenticate
// the producer and publication separately; this function grants no authority.
func ProjectReview(result, ledger []byte, expectedHead string) (Review, error) {
	var out Review
	if !sha.MatchString(expectedHead) {
		return out, fmt.Errorf("invalid_expected_head")
	}
	var fields map[string]json.RawMessage
	if err := Decode(result, &fields); err != nil {
		return out, err
	}
	var head string
	if json.Unmarshal(fields["head_sha"], &head) != nil || head != expectedHead {
		return out, fmt.Errorf("review_head_mismatch")
	}
	var l struct {
		Version  int    `json:"version"`
		Action   string `json:"action"`
		Findings *[]struct {
			ID         string          `json:"id"`
			Severity   string          `json:"severity"`
			Category   string          `json:"category"`
			File       json.RawMessage `json:"file"`
			Line       json.RawMessage `json:"line,omitempty"`
			Actionable json.RawMessage `json:"actionable,omitempty"`
		} `json:"findings"`
		Dispositions []Finding `json:"dispositions,omitempty"`
	}
	if err := Decode(ledger, &l); err != nil {
		return out, err
	}
	if l.Version != 2 || l.Findings == nil || len(*l.Findings) > 500 || len(l.Dispositions) > 500 {
		return out, fmt.Errorf("invalid_findings_ledger")
	}
	switch l.Action {
	case "approve", "request-changes", "comment", "reject":
	default:
		return out, fmt.Errorf("invalid_review_action")
	}
	var action string
	if json.Unmarshal(fields["action"], &action) != nil || action != l.Action {
		return out, fmt.Errorf("review_action_mismatch")
	}
	seen := map[string]bool{}
	out.Findings = []Finding{}
	for _, f := range *l.Findings {
		if f.ID == "" || len(f.ID) > 256 || seen[f.ID] || f.Severity == "" || f.Category == "" {
			return Review{}, fmt.Errorf("invalid_finding")
		}
		seen[f.ID] = true
		out.Findings = append(out.Findings, Finding{ID: f.ID, Status: "open"})
	}
	states := map[string]string{}
	for _, d := range l.Dispositions {
		if !seen[d.ID] || states[d.ID] != "" {
			return Review{}, fmt.Errorf("invalid_disposition")
		}
		switch d.Status {
		case "open", "resolved_by_change", "dismissed_by_human":
		default:
			return Review{}, fmt.Errorf("invalid_disposition")
		}
		states[d.ID] = d.Status
	}
	for i := range out.Findings {
		if s := states[out.Findings[i].ID]; s != "" {
			out.Findings[i].Status = s
		}
	}
	// Absent, malformed or degraded risk stays unknown, never an invented low.
	var risk map[string]json.RawMessage
	if json.Unmarshal(fields["risk_assessment"], &risk) == nil && risk != nil {
		if _, degraded := risk["degraded"]; !degraded {
			var score int
			var level string
			levels := []string{"low", "moderate", "elevated", "high", "critical"}
			if json.Unmarshal(risk["score"], &score) == nil && json.Unmarshal(risk["level"], &level) == nil && score >= 1 && score <= 5 && levels[score-1] == level {
				out.Risk = &score
			}
		}
	}
	out.Completed, out.Action = true, l.Action
	return out, nil
}
