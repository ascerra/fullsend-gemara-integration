# Security boundary

This project currently evaluates explicitly synthetic fixtures. It does not
authenticate live forge evidence, receive production credentials or request merges.
An eligible fixture result grants no merge authority. Future live adapters and
writers require their own authentication and enforcement review.

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
