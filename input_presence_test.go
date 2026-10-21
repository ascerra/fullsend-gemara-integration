// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"encoding/json"
	"testing"
)

func TestGateInputMissingOrNullFieldsCannotBecomePermissive(t *testing.T) {
	_, o, _ := judgeExample()
	_, e := baseline(t, "github", "clean")
	e.SourceKind = "collector-observation"
	cases := []struct {
		name   string
		value  any
		path   []string
		target func() any
	}{
		{"invalid", o, []string{"invalid"}, func() any { return &JudgeObservation{} }},
		{"mutation_permitted", o, []string{"mutation_permitted"}, func() any { return &JudgeObservation{} }},
		{"passed", o, []string{"passed"}, func() any { return &JudgeObservation{} }},
		{"excluded", o, []string{"excluded_other_sources"}, func() any { return &JudgeObservation{} }},
		{"draft", e, []string{"draft"}, func() any { return &Evidence{} }},
		{"bypass", e, []string{"protections", "bypass_enabled"}, func() any { return &Evidence{} }},
		{"discussions", e, []string{"protections", "discussions_resolved"}, func() any { return &Evidence{} }},
	}
	for _, tc := range cases {
		for _, null := range []bool{false, true} {
			t.Run(tc.name+map[bool]string{true: "/null", false: "/missing"}[null], func(t *testing.T) {
				raw, _ := json.Marshal(tc.value)
				var object map[string]any
				_ = json.Unmarshal(raw, &object)
				nested := object
				for _, key := range tc.path[:len(tc.path)-1] {
					nested = nested[key].(map[string]any)
				}
				key := tc.path[len(tc.path)-1]
				if null {
					nested[key] = nil
				} else {
					delete(nested, key)
				}
				raw, _ = json.Marshal(object)
				if err := Decode(raw, tc.target()); err == nil || err.Error() != "required_fields_missing_or_null" {
					t.Fatalf("missing/null accepted: %v", err)
				}
			})
		}
	}
}
