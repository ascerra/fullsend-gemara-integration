package integration

import (
	"encoding/json"
	"fmt"
)

// FixtureEnvelope contains selected synthetic forge fields, not a full API
// response. It intentionally cannot assert live authentication.
type FixtureEnvelope struct {
	Version     string         `json:"schema_version"`
	SourceKind  string         `json:"source_kind"`
	Forge       string         `json:"forge"`
	GitHub      *GitHubFixture `json:"github,omitempty"`
	GitLab      *GitLabFixture `json:"gitlab,omitempty"`
	Observation Evidence       `json:"observation"`
}

type GitHubFixture struct {
	Repository struct {
		ID       int    `json:"id"`
		FullName string `json:"full_name"`
		Host     string `json:"host"`
	} `json:"repository"`
	PullRequest struct {
		Number int    `json:"number"`
		State  string `json:"state"`
		Draft  bool   `json:"draft"`
		Head   struct {
			SHA string `json:"sha"`
		} `json:"head"`
		Base struct {
			SHA string `json:"sha"`
			Ref string `json:"ref"`
		} `json:"base"`
	} `json:"pull_request"`
}

type GitLabFixture struct {
	Project struct {
		ID   int    `json:"id"`
		Path string `json:"path_with_namespace"`
		Host string `json:"host"`
	} `json:"project"`
	MergeRequest struct {
		IID          int    `json:"iid"`
		State        string `json:"state"`
		Draft        bool   `json:"draft"`
		SHA          string `json:"sha"`
		TargetBranch string `json:"target_branch"`
	} `json:"merge_request"`
	// A current target-branch lookup supplies this value. A merge-base or
	// diff_refs.base_sha must never be substituted for the current target.
	TargetBranchSHA string `json:"target_branch_sha"`
}

// NormalizeFixture maps distinct forge subject fields into the shared contract.
// It does not fetch, authenticate, or verify any real forge data.
func NormalizeFixture(raw []byte) (Evidence, error) {
	var f FixtureEnvelope
	if err := Decode(raw, &f); err != nil {
		return Evidence{}, err
	}
	if f.Version != "forge-fixture/v1" || f.SourceKind != "synthetic-fixture" || f.Observation.SourceKind != "synthetic-fixture" {
		return Evidence{}, fmt.Errorf("fixture_marker_required")
	}
	e := f.Observation
	var identity Identity
	switch f.Forge {
	case "github":
		if f.GitHub == nil || f.GitLab != nil {
			return Evidence{}, fmt.Errorf("ambiguous_forge")
		}
		g := f.GitHub
		identity = Identity{"github", g.Repository.Host, fmt.Sprint(g.Repository.ID), g.Repository.FullName, g.PullRequest.Base.Ref}
		e.Subject.Identity = identity
		e.Subject.Change = g.PullRequest.Number
		e.Subject.Head = g.PullRequest.Head.SHA
		e.Subject.Base = g.PullRequest.Base.SHA
		e.Open = g.PullRequest.State == "open"
		e.Draft = g.PullRequest.Draft
	case "gitlab":
		if f.GitLab == nil || f.GitHub != nil {
			return Evidence{}, fmt.Errorf("ambiguous_forge")
		}
		g := f.GitLab
		identity = Identity{"gitlab", g.Project.Host, fmt.Sprint(g.Project.ID), g.Project.Path, g.MergeRequest.TargetBranch}
		e.Subject.Identity = identity
		e.Subject.Change = g.MergeRequest.IID
		e.Subject.Head = g.MergeRequest.SHA
		e.Subject.Base = g.TargetBranchSHA
		e.Open = g.MergeRequest.State == "opened"
		e.Draft = g.MergeRequest.Draft
	default:
		return Evidence{}, fmt.Errorf("unsupported_forge")
	}
	// Reject a fixture that supplied conflicting normalized subject data.
	a, _ := json.Marshal(e.Subject)
	b, _ := json.Marshal(f.Observation.Subject)
	if string(a) != string(b) {
		return Evidence{}, fmt.Errorf("conflicting_fixture_subject")
	}
	return e, nil
}
