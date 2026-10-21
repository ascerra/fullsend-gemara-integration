// SPDX-License-Identifier: Apache-2.0

package integration

import "time"

// CandidateAssessment is assessment data, never a merge authorization receipt.
type CandidateAssessment struct {
	Version           string     `json:"schema_version"`
	Decision          string     `json:"decision"`
	Authorization     string     `json:"authorization"`
	MutationPermitted bool       `json:"mutation_permitted"`
	Candidate         Report     `json:"candidate"`
	Quality           JudgeCheck `json:"quality"`
}

// AssessCandidate requires all five PR requirements AND the quality requirement.
// Passing quality never overrides a failing candidate requirement.
func AssessCandidate(b Binding, e Evidence, p JudgePolicy, o JudgeObservation, now time.Time) (CandidateAssessment, error) {
	candidate, err := EvaluateCandidate(b, e, now)
	if err != nil {
		return CandidateAssessment{}, err
	}
	quality, err := CheckJudge(p, o, now)
	if err != nil {
		return CandidateAssessment{}, err
	}
	decision := "hold"
	if candidate.Decision == "eligible" && quality.Decision == "eligible" {
		decision = "eligible"
	}
	return CandidateAssessment{Version: "candidate-assessment/v1", Decision: decision, Authorization: "none", Candidate: candidate, Quality: quality}, nil
}
