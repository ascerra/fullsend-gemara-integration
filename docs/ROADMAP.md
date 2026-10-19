# Implementation roadmap

## Completed foundation

- Separate reusable repository and owner bindings; no change-size rule.
- Five controls bound to native Gemara SDK assessments and native EvaluationLogs.
- GitHub/GitLab fixture parity with identical shared artifact digests.
- Guard tests for revision, policy, identity, freshness, scope, checks, findings,
  owner authorization and protection changes.
- Explicit fixture-only authority; no live writer or implicit deployment.

## Next: prove the normal Fullsend baseline

1. Select a tested official runtime/agent/image combination through normal installation.
2. Verify PR creation, new push and manual Review on auto-merge.
3. Inventory compatibility differences before moving anything into this library.
4. Verify a legitimate human-reviewed route for governance changes under the
   existing required queue check. Do not use an admin bypass as the demo path.

## Then: authenticated GitHub integration

1. Consume the public Apache-2.0 integration at an owner-approved immutable commit.
2. Add the smallest supported post-script publication extension, preserving
   official Review behavior and complete finding dispositions.
3. Implement authenticated, subject-bound GitHub evidence collection. Extend the
   contract explicitly rather than marking fixtures as trusted live records.
4. Add required target-controlled evaluation and a separately scoped writer,
   with immediate pre-write and actual merge-group revalidation.
5. Prove a hosted eligible case and a hosted hold without bypass or retry tricks.
6. Replace the embedded implementation in auto-merge with pinned consumption;
   remove retired code only after the replacement is proven.

## GitLab, after separate project authorization

Implement real GitLab collection and required gating, first observe-only, then
prove a real MR pass/hold and protected cumulative-result enforcement. Keep its
repository policy, credentials and approvals under that project's owner control.

## Decisions still needed

- Normal governance-change route for auto-merge before deploying its consumer migration.

No new upstream Fullsend/agents work or GitLab changes are authorized by creating
this repository.
