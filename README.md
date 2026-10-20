# Fullsend + Gemara integration

**An owner policy that travels between repositories.** Fullsend supplies Review;
our checks evaluate owner requirements using the Gemara SDK and produce native
EvaluationLogs. Each consuming owner supplies repository-specific configuration.

Licensed under [Apache-2.0](LICENSE). Consumers can fetch the public source at a
reviewed immutable commit without a distribution credential.

**Current status: working synthetic demonstration, not a deployed merger.**
The repository includes the standard Fullsend installation scaffold.
The Gemara evaluator still has no live forge authentication, policy publisher,
or merge writer. `eligible` means that a fixture satisfies
the candidate requirements; it grants no permission to merge. Existing auto-merge
and GitLab deployments are unchanged.

## Fullsend agents

The per-repository Fullsend shim calls the official v0.48.1 workflows at immutable
commit `0cb88dc74260785f0b977f7277f91c0039eaaf21`. The standard triage, coder,
review, fix, retro, and prioritize roles are enabled. Fix uses the coder app.
The standard prioritization companion workflow is included.

Agent execution requires the Fullsend triage, coder, review, retro, and prioritize
GitHub Apps to have access to this repository, along with configured Actions
inference secrets and variables.
Inference uses repository-scoped Google workload identity federation; no
service-account key belongs in the repository or its Actions secrets.

Workflow installation does not establish a live Gemara gate, publish EvaluationLogs,
or authorize merging. Those remain separate implementation and acceptance steps.

## Try it

With a supported, patched Go release (CI uses Go 1.26.9), from this checkout:

```sh
go run ./cmd/gemara-owner demo
```

The same embedded catalog, policy and evaluator run 18 measured cases across
GitHub and GitLab fixture projections. The demo uses a fixed test time of
October 19, 2026 at 18:00 UTC. No forge or model calls are made. Go may download
the pinned SDK dependencies on the first build.

- Allowed change → eligible.
- Change to governed paths, even one file → hold.
- Larger allowed change (120 files) → eligible.
- Open finding, old revision, failed/missing check, wrong project, expired grant → hold.

There is no line-count eligibility rule. The governed-change fixture represents
the one-line attack scenario through its changed path; this evaluator does not
inspect or use a diff's length.

## The five requirements

1. **Within my delegation:** owner-approved paths and change kinds, with an active grant.
2. **No unresolved review concerns:** approval, acceptable risk, complete finding state.
3. **Required checks pass:** the configured checks succeed for this revision.
4. **Evidence belongs to this revision:** consistent project, head, target, policy and publication context; current observations.
5. **Protected merge requirements hold:** required gate, protected target, resolved discussions and no bypass.

The demo checks synthetic claims for these conditions. The live adapters still
have to authenticate those claims. Final merged-result/queue enforcement is a
separate acceptance test, not something the current candidate fixtures prove.

Gemara does not infer safety from policy prose. The [common policy](policy/common.json)
binds named checks to the [control catalog](policy/catalog.json); our implementation
performs the checks, and Gemara runs the assessments and records their evidence.

## What belongs where?

| Shared here | Owned by each consumer |
| --- | --- |
| Catalog, common policy, evaluator | Allowed paths/change types and acceptable risk |
| Evidence contract and adapter implementation | Forge host/project, trusted Review workflow, required checks |
| Future publication/writer implementation | Credentials, protections and permission to execute a writer |
| SDK/schema pins, tests and installation recipes | Integration revision pin and owner grant |

Compare the two consumer examples:

- [GitHub binding](examples/github/binding.json): illustrative auto-merge configuration.
- [GitLab binding](examples/gitlab/binding.json): a synthetic example project, not an internal deployment.

Both use identical common-policy and catalog digests. No code is copied into
the consumer examples. [Boundary and adoption guide](docs/ADOPTION.md).

## Inspect the native log

```sh
go run ./cmd/gemara-owner evaluate-fixture \
  --binding examples/gitlab/binding.json \
  --fixture testdata/gitlab/clean.json \
  --at 2026-10-19T18:00:00Z
```

Held evaluations exit with status 2. Reports include the policy, catalog and
binding digests, all five assessment results, and the native EvaluationLog.
Malformed inputs also fail with a nonzero status.

## Verify

```sh
go test ./...
go vet ./...
mkdir -p out
go run ./cmd/gemara-owner demo --json > out/results.json
python3 scripts/validate_artifacts.py out/results.json
```

The optional schema check downloads hash-pinned public CUE and official Gemara
schema archives on Linux x86-64. It validates both documents and all 18 native logs.
The Go tests and evaluator do not require CUE.

The SDK is pinned to commit `5e33fda460a80ffa99e9efbe19afb43e7d4c4dd2`, matching
Gemara schema 1.5.0. [Provenance and licensing](NOTICE.md).

## Next

Establish ordinary Fullsend Review as a baseline, add the smallest supported
publication extension, implement authenticated GitHub evidence collection, then
prove a hosted pass and hold. GitLab fixtures prove shared policy behavior;
GitLab authentication, required merge gating and protected merging remain future
work. [Implementation roadmap](docs/ROADMAP.md).
