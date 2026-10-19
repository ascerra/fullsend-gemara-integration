# Provenance and licensing

This is an experimental integration using Fullsend and Gemara. It is not an
official release of either project and does not replace their installation flow.

The owner-policy separation and refusal cases originate in the
[`fullsend-ai/auto-merge`](https://github.com/fullsend-ai/auto-merge) integration lab,
whose deployed reference at creation was commit
`3b529bb9128438c71218a602fc7a746b0be092a2`. This repository implements a smaller,
fixture-only shared core; it does not copy that repository's live merge writer,
credentials, internal operational data or historic workflow artifacts.

Native assessment execution uses
[`gemaraproj/go-gemara`](https://github.com/gemaraproj/go-gemara) at commit
`5e33fda460a80ffa99e9efbe19afb43e7d4c4dd2` and schema version 1.5.0.
Official schemas are verified against Gemara commit
`9d36c253484d14922010252bfffe58bdcd49a144`; the optional validator uses CUE v0.17.1.
Their licenses remain with their respective projects. Downloaded schema LICENSE
files are retained in the ignored tool cache, not silently stripped.

Unless otherwise noted, this integration's source code and documentation are
licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for the
complete terms. Third-party dependencies and downloaded tools retain their own
licenses; this grant does not relicense them.
