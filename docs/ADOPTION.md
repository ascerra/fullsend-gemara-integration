# Reuse boundary and adoption

## Shared integration

This repository owns the five control definitions, common Gemara policy, typed
evidence contract, evaluator, fixture normalizers, and tests. Future official
Review publication helpers and forge adapters belong here too. There is no
separate plugin system or custom Fullsend runtime.

## Consumer configuration

Each owner approves a binding with:

- Exact forge host, immutable project ID, repository path and target branch.
- Allowed paths and governed paths. Exact paths and `directory/**` are supported;
  governed paths take precedence. The initial supported change kind is a modified
  regular file. Renames, additions, deletions and symlinks hold until explicitly
  implemented and tested. Change size is not a policy input.
- Required check names and trusted Review workflow identity.
- Acceptable risk and evidence freshness.
- An enabled, expiring owner grant.

The examples are configuration fixtures, not deployed grants. Changing a binding
changes its digest and invalidates observations bound to the old one. A future
collector must load the binding from a trusted target-side source, never from
the candidate PR/MR. No tokens or private keys belong in bindings.

The report records both shared-policy and binding digests. Owner configuration
parameterizes the named checks; it is not itself a Gemara Policy document. A
future repository-specific Gemara policy can import the common policy if richer
governance is needed, without changing its shared control meanings.

## Evidence and authority

`gemara-owner project-review --result FILE --ledger FILE --head SHA` converts the
official Review result and post-script v2 findings ledger into the same review
fields for either forge. Run it only after the official post-script successfully
publishes. It preserves open findings, resolved dispositions and unknown risk;
an absent ledger is an error. The head must come from independent host context.
The result has no merge authority and does not authenticate its input files.
Consumer wiring must capture the actual post-script ledger, authenticate the
job/artifact, and establish current head/target, checks and protections separately.
Do not reconstruct a ledger by scraping narrative review text.

`gemara-owner check-review --projection FILE --head SHA --maximum-risk 2` applies
the same native Gemara `REVIEW-CLEAR` assessment to a collector-produced projection.
It exits 2 on hold and retains a native assessment with `risk_assessment_missing`
when the score is absent. This evaluates one control, not the entire merge policy.
The caller must authenticate the producer and establish the expected head first.
The report has no merge authority and does not claim to authenticate the input.

Run the shared executable in a separate CI job with no write credentials or
inference secrets. Clearing child environment variables inside a privileged
post-script is not process isolation: the same UID can still access its parent.
Keep authenticated collection in trusted owner-controlled code and pass only
bounded data to the assessment job. Do not execute downloaded shared code inside
the privileged official Review post-script.

`FixtureEnvelope` contains selected synthetic forge fields plus an observation.
The GitHub and GitLab normalizers map distinct project and change identities into
one `Subject`; conflicting duplicate identities are rejected. For GitLab the
current target-branch revision is explicit: a merge-base or diff base is not a
substitute for it. These are fixture normalizers, not live API clients.

The evaluator checks the subject, binding/policy digests, observation freshness,
publication metadata, finding state, check results, scope and protection claims.
It runs real SDK assessments and emits a native EvaluationLog. `EVIDENCE-CURRENT`
means coherent synthetic metadata here, not proof that a live forge authenticated it.

Live adapters must establish authenticated producer/run/artifact identity and
revision binding before evaluation. They must reject raw MR/PR text as evidence,
verify completeness and duplicate identities, and map finding dispositions
without losing unresolved concerns. Human dismissals currently remain unknown
because this fixture contract contains no authenticated disposition proof.

Final target freshness, required gate enforcement, cumulative merge-result
validation, and immediate pre-write revalidation belong at the enforcement
boundary. This library has no network credentials or merge operation. Every
current output explicitly says `authorization: none` and `mutation_permitted: false`.

## GitHub adoption

Keep the ordinary Fullsend install, official dispatcher/runtime and inherited
Review. Add only the supported publication seam required for bounded evidence.
Verify automatic reviews, new pushes and `/fs-review`. Document any remaining
patch or compatibility setting independently of Gemara.

Consume a reviewed immutable commit of this public Apache-2.0 integration.
Downloading the shared source requires no distribution credential. Pin the
version in owner-controlled configuration and build without forge credentials;
do not fetch a candidate PR's chosen version. Live collection and writing still
require their separately scoped permissions. Do not reuse merge credentials for
dependency downloads or expose credentials to PR code.

An authenticated collector, required policy gate and scoped writer are still
needed. Do not point the existing live merger at fixture outputs.

## GitLab adoption

Keep the standard Fullsend GitLab install and normal runner. Confirm that Review
works on real MRs before extending its publication. A working Scribe or standalone
Review pipeline does not establish a merge gate.

Reuse the exact common policy, catalog and evaluator revision; supply a local
binding and implement GitLab-specific authentication of Review jobs, pipelines,
artifacts, discussions and protections. Begin in observe-only mode.

Install a required gate that MR code cannot omit, weaken or mark optional. Verify
the instance's feature availability, configuration and token roles. If merge
trains are enabled, test the actual cumulative candidate result; source-head
binding alone is insufficient. The current fixture expects merge-train protection
but proves neither its availability nor its enforcement on a real project.

No GitLab project is modified or contacted by this repository's tests.
