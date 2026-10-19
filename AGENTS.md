# Integration repository instructions

- Keep shared requirements and evaluation independent of forge and repository.
- Repository-specific paths, checks, identities, and owner grants belong in consumer bindings.
- Never use change size as an eligibility signal.
- Keep synthetic fixtures visibly distinct from authenticated live evidence.
- Evaluation is read-only. Do not add a merge writer without an explicit scope and authority review.
- Preserve normal Fullsend installation and supported harness extension mechanisms.
- Do not change upstream Fullsend, agents, auto-merge, or GitLab repositories as a side effect of this repository's tests.
- Never commit credentials, real event payloads, or private operational data.
- Run `go test ./...` and `go vet ./...` before committing Go changes.
- Commits require user authorization and a DCO sign-off.
