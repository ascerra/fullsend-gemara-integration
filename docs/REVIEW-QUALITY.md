# Experimental review quality requirement

An otherwise eligible PR must also wait when the review system's observed
judged-approval pass rate falls below the owner's threshold. This is an
additional required condition: it never overrides a failed PR check, missing
risk assessment, finding, owner grant or protected merge requirement.

The shared `REVIEW-QUALITY` catalog and policy bind our deterministic check to a
native Gemara assessment. Gemara records Passed, Failed or Unknown; it does not
run the judge, establish MLflow authenticity, or merge a PR.

## Try the boundary

```sh
go run ./cmd/gemara-owner check-judge \
  --policy examples/quality/policy.json \
  --observation examples/quality/below-threshold.json \
  --at 2026-10-20T00:00:00Z
```

The synthetic 94/100 example returns Failed / hold and exits 2. Replace the
observation with `at-threshold.json`: exactly 99/100 passes and exits 0. These
fixtures are not live evidence. No threshold is relaxed to make a demo pass.

## Shared versus consumer

Shared: catalog, policy, `CheckJudge`, CLI, bounded read-only MLflow collector,
validation, native results and synthetic examples.

Consumer: trusted endpoint, reader credentials, experiment/source identity,
window, sample minimum, threshold, freshness limits, current candidate binding,
shared source pin and protected enforcement. Load these from the protected base,
never a candidate branch. Run collection with only reader credentials, then
assess aggregate data in a separate job with no credentials. A normalized JSON
file and its digest are not proof of authentication by themselves.

The sample binding uses a trailing 365-day trace-start window, minimum 100
scored approvals, exact 9900 basis points (99%), snapshot age at most 15 minutes,
and latest matching assessment at most seven days old. These are experimental
owner choices, not statistically validated recommendations. Configure the same
window and scorer/source in both collector and policy. Snapshot freshness does
not turn old review evidence into evidence for the current PR.

```sh
python3 scripts/collect_mlflow_judge.py \
  --binding /trusted/collector-binding.json --output /private/observation.json
```

Credentials are read only from `MLFLOW_READER_USERNAME` and
`MLFLOW_READER_PASSWORD`. The collector uses HTTPS, rejects redirects, caps
responses and pagination, verifies experiment/window, rejects duplicate traces and
accepts one valid binary assessment per completed trace from the configured
source. It calls metadata search only; it never fetches trace artifacts, runs
a model, or writes feedback. Only counts, timestamps and a digest leave memory.
The collection time is truncated to whole seconds before filtering. Two scans
with the same fixed window must produce identical aggregates and record digests;
a changed population or duplicate trace aborts collection. This detects common
pagination races but is not database snapshot isolation under concurrent writes.
Do not upload local live aggregates to public repositories without approval.

The aggregate is explicitly **experiment-wide scored approvals**. It is not a
per-repository or current-agent-version cohort. Different review versions and
rubrics can be mixed in that experiment; the source ID alone does not pin a
judge revision. Production use needs reliable cohort/version identity, coverage
measurement and validation against independent human labels.

## Fail closed and compose

Below threshold is Failed. Missing, stale, incomplete, ambiguous, malformed or
insufficient data is Unknown. Both mean hold, including MLflow outages. Empty
or unscored populations never count as 100%. Count comparisons use integer
arithmetic, so rounding a dashboard value cannot produce a pass.

A consuming merge authorizer must require **all five candidate controls AND
REVIEW-QUALITY**, bind every result to current trusted configuration, and
recheck freshness at enrollment and queue validation. Preserve the existing
protected merge writer. Never execute shared code beside its write credential
or trust candidate-provided reports. Do not mark this advisory check enforced
until a live negative candidate cannot merge through the actual writer/queue.

## What the number means

`review_correctly_approved` scores justified approvals as 1 and false greens as
0. Non-approvals and insufficient evidence can be skipped. The observed mean
is the judge's pass rate among scored approvals, not the review agent's overall
accuracy, coverage, or a calibrated probability that the next PR is safe.
Mechanical failures may share the assessment source with soft LLM judgments.
A 99% observed pass rate is not a 99% confidence bound or production guarantee.

The `assess-candidate` command requires `--binding`, `--observation`,
`--judge-policy`, `--judge-observation` and `--at`. It produces both native
assessment sets and an AND-composed decision. Only collector observations are
accepted; the fixture API remains separate. Passing this API's data checks is
not a claim that authentication or live merge enforcement has been deployed.

The demo deliberately uses a strict invalid-assessment rule: any bound-source
invalidated, overridden, ambiguous, non-binary or errored assessment holds the
population, including nested feedback errors. Such rows are not silently removed
from the denominator or mislabeled as another source. This can hold all candidates
for the full window. Recovery requires the data owner to correct the source
assessment or explicitly approve a different versioned cohort policy; this
collector never edits feedback, chooses a convenient rescore or relaxes policy.

A [synthetic candidate observation](../examples/quality/synthetic-candidate-observation.json)
exists only to exercise the CLI at a fixed test time. Despite its API shape, it
is not authenticated live data and must never be consumed by a merge writer.
