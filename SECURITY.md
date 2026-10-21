# Security boundary

This project evaluates synthetic fixtures and caller-authenticated collector
observations. The optional MLflow collector reads reader credentials from its
environment and performs only bounded metadata searches; it exports aggregates
without trace prose or credentials. It does not authenticate forge evidence,
run a judge, or request merges. Assessment results grant no merge authority.
Consumer adapters and protected enforcement require their own security review.

CI uses a patched Go 1.26 release, verifies module hashes, scans the complete Git
history using checksum-pinned Gitleaks, checks reachable vulnerabilities with the
pinned official Go scanner, and validates native logs against pinned schemas.
Jobs have read-only repository permissions, no persisted checkout credentials,
no dependency cache sharing and no deployment or merge credentials.

The initial full-history audit found no credential leaks. The source scan with
Go 1.26.6 found no reachable vulnerable calls, but reported standard-library
advisories, including a Windows-only `os.Root` issue in an imported package.
The repeat scan with Go 1.26.9 reported no vulnerabilities. CI was updated to
that patched release. Vulnerability reachability and scanner coverage are
limits of the audit, not a guarantee that the project is free of defects.

Never report a credential value in a public issue, PR, test fixture or CI log.
If a credential is accidentally published, revoke it before arranging any history
cleanup. Do not weaken scanner rules to hide an unexplained finding.
