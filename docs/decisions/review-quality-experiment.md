# Owner decision: experimental shared review quality assessment

Status: owner-approved implementation and hosted testing; conditional normal
merge approval after latest CI and Fullsend reviews pass and findings are resolved.

The repository owner requested a reusable Gemara check of the existing
`review_correctly_approved` score at a 99% threshold. The owner approved DCO
commits, pushes, PR tests and necessary tested fixes, then explicitly answered
"Approve conditional normal merges" for shared PR #8 and auto-merge PR #52.
This record preserves that scope; it is not independent review evidence.

Approved protected-path scope in this repository:

- `.github/workflows/ci.yml`: run the new collector's synthetic Python tests.
- `scripts/collect_mlflow_judge.py`: optional bounded metadata-only HTTPS reader;
  no MLflow writes, judge calls, trace artifacts or merge operations.
- `scripts/test_collect_mlflow_judge.py`: tests with synthetic data and mocked APIs.
- `scripts/validate_artifacts.py`: validate the added Gemara catalog and policy.

The shared requirement, CLI and documentation remain experimental. Credentials
and real evidence are not committed. The demo requires the configured threshold
and fail-closed source, completeness, sample and freshness checks. Findings must
be fixed, then reviewed and tested on the actual merge candidate.

This does not approve a bypass, a protection change, a new merge writer or enabling
the consumer gate. Those are outside the conditional approval. Repository-visible
human approval and all normal controls remain required; this document does not
supply a GitHub review, self-approve deployment or override a failed assessment.
