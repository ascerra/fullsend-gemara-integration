// SPDX-License-Identifier: Apache-2.0

// Package integration evaluates owner requirements using the Gemara SDK.
// This initial version accepts synthetic fixture projections only and has no writer.
package integration

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"
	"time"
)

type Identity struct {
	Forge        string `json:"forge"`
	Host         string `json:"host"`
	ProjectID    string `json:"project_id"`
	Repository   string `json:"repository"`
	TargetBranch string `json:"target_branch"`
}

// Binding is owner-controlled consumer configuration, never read from PR/MR code.
type Binding struct {
	Version           string   `json:"schema_version"`
	Identity          Identity `json:"identity"`
	AllowedPaths      []string `json:"allowed_paths"`
	GovernedPaths     []string `json:"governed_paths"`
	RequiredChecks    []string `json:"required_checks"`
	ReviewWorkflow    string   `json:"review_workflow"`
	MaximumRisk       int      `json:"maximum_risk"`
	MaximumAgeSeconds int      `json:"maximum_evidence_age_seconds"`
	GrantEnabled      bool     `json:"owner_grant_enabled"`
	GrantExpiresAt    string   `json:"owner_grant_expires_at"`
}

type Subject struct {
	Identity      Identity `json:"identity"`
	Change        int      `json:"change"`
	Head          string   `json:"head_sha"`
	Base          string   `json:"target_sha"`
	PolicyDigest  string   `json:"policy_sha256"`
	BindingDigest string   `json:"binding_sha256"`
}

type ChangedFile struct {
	Path         string `json:"path"`
	PreviousPath string `json:"previous_path,omitempty"`
	Kind         string `json:"kind"`
	Mode         string `json:"mode"`
}

type Finding struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type Review struct {
	Completed bool      `json:"completed"`
	Action    string    `json:"action"`
	Risk      *int      `json:"risk_score"`
	Findings  []Finding `json:"findings"`
}

type Check struct {
	Name   string `json:"name"`
	Head   string `json:"head_sha"`
	Status string `json:"status"`
}

type Publication struct {
	Workflow       string  `json:"workflow"`
	Run            string  `json:"run_id"`
	Attempt        int     `json:"attempt"`
	Subject        Subject `json:"subject"`
	ArtifactDigest string  `json:"artifact_sha256"`
}

type Protections struct {
	ProtectedTarget     bool   `json:"protected_target"`
	RequiredPolicyGate  bool   `json:"required_policy_gate"`
	DiscussionsResolved bool   `json:"discussions_resolved"`
	BypassEnabled       bool   `json:"bypass_enabled"`
	Mechanism           string `json:"merge_mechanism"`
}

// Evidence is a normalized fixture, not a live attestation. Its source marker
// cannot be promoted to authenticated evidence by changing an input field.
type Evidence struct {
	SourceKind  string        `json:"source_kind"`
	Subject     Subject       `json:"subject"`
	CollectedAt string        `json:"collected_at"`
	Open        bool          `json:"open"`
	Draft       bool          `json:"draft"`
	Files       []ChangedFile `json:"files"`
	Review      Review        `json:"review"`
	Checks      []Check       `json:"checks"`
	Publication Publication   `json:"publication"`
	Protections Protections   `json:"protections"`
}

var sha = regexp.MustCompile(`^[a-f0-9]{40}$`)
var hash = regexp.MustCompile(`^[a-f0-9]{64}$`)
var host = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9.-]{0,251}[a-z0-9])?$`)
var project = regexp.MustCompile(`^[1-9][0-9]{0,14}$`)

func Digest(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		panic("unsupported digest input")
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// Decode rejects duplicate keys as well as unknown fields. A policy field must
// never have two possible meanings depending on which JSON reader sees it.
func Decode(raw []byte, target any) error {
	if len(raw) == 0 || len(raw) > 1024*1024 {
		return fmt.Errorf("input_size_invalid")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	if err := keys(d, 0); err != nil {
		return fmt.Errorf("invalid_json")
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("trailing_json")
	}
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return fmt.Errorf("invalid_fields")
	}
	return nil
}

func keys(d *json.Decoder, depth int) error {
	if depth > 24 {
		return fmt.Errorf("depth")
	}
	token, err := d.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			k, err := d.Token()
			if err != nil {
				return err
			}
			name, ok := k.(string)
			if !ok || seen[name] {
				return fmt.Errorf("duplicate")
			}
			seen[name] = true
			if err := keys(d, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := keys(d, depth+1); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("delimiter")
	}
	_, err = d.Token()
	return err
}

func validPath(p string) bool {
	return p != "" && !strings.ContainsAny(p, "\\\x00\r\n") && !strings.HasPrefix(p, "/") && path.Clean(p) == p && p != "." && p != ".." && !strings.HasPrefix(p, "../")
}

func (b Binding) Validate() error {
	if b.Version != "owner-binding/v1" || (b.Identity.Forge != "github" && b.Identity.Forge != "gitlab") || !host.MatchString(b.Identity.Host) || !project.MatchString(b.Identity.ProjectID) || !validPath(b.Identity.Repository) || b.Identity.TargetBranch == "" {
		return fmt.Errorf("invalid_binding_identity")
	}
	if b.MaximumRisk < 1 || b.MaximumRisk > 5 || b.MaximumAgeSeconds < 1 || b.MaximumAgeSeconds > 3600 || b.ReviewWorkflow == "" {
		return fmt.Errorf("invalid_binding_limits")
	}
	if _, err := time.Parse(time.RFC3339, b.GrantExpiresAt); err != nil {
		return fmt.Errorf("invalid_owner_grant")
	}
	for _, values := range [][]string{b.AllowedPaths, b.GovernedPaths, b.RequiredChecks} {
		if len(values) == 0 || len(values) > 100 {
			return fmt.Errorf("invalid_binding_list")
		}
		seen := map[string]bool{}
		for _, v := range values {
			if v == "" || len(v) > 256 || seen[v] {
				return fmt.Errorf("invalid_binding_list")
			}
			seen[v] = true
		}
	}
	for _, p := range append(append([]string{}, b.AllowedPaths...), b.GovernedPaths...) {
		if !validPath(strings.TrimSuffix(p, "/**")) || strings.ContainsAny(strings.TrimSuffix(p, "/**"), "*?[") {
			return fmt.Errorf("invalid_path_rule")
		}
	}
	return nil
}

func matches(p string, rules []string) bool {
	for _, rule := range rules {
		if p == rule {
			return true
		}
		if strings.HasSuffix(rule, "/**") {
			prefix := strings.TrimSuffix(rule, "/**")
			if p == prefix || strings.HasPrefix(p, prefix+"/") {
				return true
			}
		}
	}
	return false
}
