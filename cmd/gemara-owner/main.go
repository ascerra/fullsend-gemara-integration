// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	integration "github.com/ascerra/fullsend-gemara-integration"
)

func read(p string) ([]byte, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, fmt.Errorf("input_unavailable")
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, 1024*1024+1))
	if err != nil || len(raw) > 1024*1024 {
		return nil, fmt.Errorf("input_size_invalid")
	}
	return raw, nil
}

func evaluate(binding, fixture, scenario string, now time.Time) (integration.Report, error) {
	raw, err := read(binding)
	if err != nil {
		return integration.Report{}, err
	}
	var b integration.Binding
	if err := integration.Decode(raw, &b); err != nil {
		return integration.Report{}, err
	}
	raw, err = read(fixture)
	if err != nil {
		return integration.Report{}, err
	}
	e, err := integration.NormalizeFixture(raw)
	if err != nil {
		return integration.Report{}, err
	}
	if scenario != "" {
		b, e, err = integration.FixtureCase(b, e, scenario)
		if err != nil {
			return integration.Report{}, err
		}
	}
	return integration.EvaluateFixture(b, e, now)
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: gemara-owner demo | evaluate-fixture | project-review (use -h for flags)")
	}
	switch os.Args[1] {
	case "project-review":
		fs := flag.NewFlagSet("project-review", flag.ContinueOnError)
		result := fs.String("result", "", "official Review result file")
		ledger := fs.String("ledger", "", "official post-script v2 findings ledger")
		head := fs.String("head", "", "independently established reviewed head")
		if err := fs.Parse(os.Args[2:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("unexpected_arguments")
		}
		rawResult, err := read(*result)
		if err != nil {
			return err
		}
		rawLedger, err := read(*ledger)
		if err != nil {
			return err
		}
		r, err := integration.ProjectReview(rawResult, rawLedger, *head)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(struct {
			Version           string             `json:"schema_version"`
			Head              string             `json:"head_sha"`
			Review            integration.Review `json:"review"`
			Authorization     string             `json:"authorization"`
			MutationPermitted bool               `json:"mutation_permitted"`
		}{"review-projection/v1", *head, r, "none", false})
	case "demo":
		fs := flag.NewFlagSet("demo", flag.ContinueOnError)
		root := fs.String("root", ".", "integration checkout")
		jsonMode := fs.Bool("json", false, "emit native logs and assessment data")
		if err := fs.Parse(os.Args[2:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("unexpected_arguments")
		}
		now, _ := time.Parse(time.RFC3339, "2026-10-19T18:00:00Z")
		cases := integration.DemoCaseNames
		type item struct {
			Forge  string             `json:"forge"`
			Case   string             `json:"case"`
			Report integration.Report `json:"report"`
		}
		var all []item
		for _, forge := range []string{"github", "gitlab"} {
			for _, name := range cases {
				binding := filepath.Join(*root, "examples", forge, "binding.json")
				r, err := evaluate(binding, filepath.Join(*root, "testdata", forge, "clean.json"), name, now)
				if err != nil {
					return fmt.Errorf("demo_%s_%s: %w", forge, name, err)
				}
				expected := "hold"
				if name == "clean" || name == "larger-allowed" {
					expected = "eligible"
				}
				if r.Decision != expected {
					return fmt.Errorf("demo_expectation_failed_%s_%s", forge, name)
				}
				all = append(all, item{forge, name, r})
				if !*jsonMode {
					fmt.Printf("%-6s %-22s %-8s", forge, name, r.Decision)
					for _, a := range r.Assessments {
						if a.State != "passed" {
							fmt.Printf(" %s=%s (%s)", a.ID, a.State, a.Reason)
						}
					}
					fmt.Println()
				}
			}
		}
		if *jsonMode {
			return json.NewEncoder(os.Stdout).Encode(all)
		}
		fmt.Println("18 measured synthetic cases; native Gemara SDK; identical shared policy; no live authentication or merge authority.")
		return nil
	case "evaluate-fixture":
		fs := flag.NewFlagSet("evaluate-fixture", flag.ContinueOnError)
		binding := fs.String("binding", "", "owner binding")
		fixture := fs.String("fixture", "", "synthetic forge fixture")
		at := fs.String("at", "", "explicit evaluation time")
		if err := fs.Parse(os.Args[2:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("unexpected_arguments")
		}
		now, err := time.Parse(time.RFC3339, *at)
		if err != nil {
			return fmt.Errorf("explicit_time_required")
		}
		r, err := evaluate(*binding, *fixture, "", now)
		if err != nil {
			return err
		}
		if err := json.NewEncoder(os.Stdout).Encode(r); err != nil {
			return err
		}
		if r.Decision != "eligible" {
			return fmt.Errorf("policy_held")
		}
		return nil
	default:
		return fmt.Errorf("unsupported_command")
	}
}

func main() {
	if err := run(); err != nil {
		// Do not copy input contents, filesystem errors, or environment data into CI.
		code := err.Error()
		if len(code) > 250 || strings.ContainsAny(code, "\r\n") {
			code = "evaluation_failed"
		}
		fmt.Fprintln(os.Stderr, code)
		os.Exit(2)
	}
}
