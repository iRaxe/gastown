# gt 1.2 Subepic Branch Candidates

Prepared: 2026-05-23 21:35 UTC

Scope: branch packaging preparation for `gt-12-clean-subepic-pr-branches`. No maintainer-facing PRs were opened.

## Base And Target

- Upstream PR base: `origin/main` at `625bcf8a`.
- Merge-queue target for this manifest: `origin/integration/gt-1-2-convergence-cleanup` at `a463ae52`.
- Construction method: create one clean branch per selected source leaf from `origin/main`, then `git merge --squash <source>` and commit once.
- Remote-facing branch namespace: `polecat/ghoul/gt-1-2-candidate-*` because the fork pre-push guard rejects non-`polecat/*` agent branches.

## Candidate Branches

| Subepic | Clean candidate branch | Candidate tip | Canonical source | Source tip | Result |
| --- | --- | --- | --- | --- | --- |
| Routing identity | `polecat/ghoul/gt-1-2-candidate-routing-identity` | `2023d923` | `origin/integration/gt-1-2-routing-identity-gate-identity` | `21c5d924` | One squashed commit; tree matches source. |
| MR target/source | `polecat/ghoul/gt-1-2-candidate-mr-target-source` | `3d3ecddf` | `origin/integration/gt-1-2-mr-target-and-source-transition-gate-source` | `2718682b` | One squashed commit; tree matches source. |
| Polecat workstate | `polecat/ghoul/gt-1-2-candidate-polecat-workstate` | `26414364` | `origin/integration/gt-1-2-canonical-polecat-workstate-workstate` | `1d3e6039` | One squashed commit; tree matches source. |
| Recovery false positives | `polecat/ghoul/gt-1-2-candidate-recovery-false-positives` | `bc1783c6` | `origin/integration/gt-1-2-recovery-false-positive-matrix-positives` | `fa5a9da9` | One squashed commit; tree matches source. |
| Capacity/admission | `polecat/ghoul/gt-1-2-candidate-capacity-admission` | `ccda9074` | `origin/integration/gt-1-2-capacity-and-admission-gate-admission` | `aa3ade02` | One squashed commit; tree matches source. |
| Notification actionability | `polecat/ghoul/gt-1-2-candidate-notification-actionability` | `468b84a7` | `origin/integration/gt-1-2-notification-actionability-gate-actionability` | `ede0d98a` | One squashed commit; tree matches source. |

## Deferred Or Dropped Subepics

| Subepic | Source ref | Rationale |
| --- | --- | --- |
| Release candidate/canary | `origin/integration/gt-1-2-release-candidate-and-canary-gate-canary` at `625bcf8a` | Deferred: source equals `origin/main`, has zero delta, and would produce an empty package branch. |
| Coordination/release evidence | `fork/integration/gt-1-2-coordination-and-release-inventory-inventory` at `625bcf8a` from topology audit | Dropped for this packaging pass: zero-delta evidence-only ref, no package content. |

## Explicit Exclusions

- Unsuffixed stale integration branches at `b381f60a`: excluded as old integration heads with unrelated convergence history.
- Wrong-target `integration/test-beaddolt-hardenning` PR artifacts `#4084`-`#4089`, `#4092`, `#4109`, and `#4114`: excluded from package input selection.
- WIP/autosave/checkpoint branch heads: excluded as direct package branches. Their selected source leaves were squashed into one clean commit per candidate instead.
- Fork short-name duplicates: not double-counted; source selection follows the topology audit's canonical `origin/integration/gt-1-2-*` leaves.
- Existing open main-target PRs `#4080`, `#4081`, and `#4096`: not adopted automatically; they remain review inputs for the later package gate.

## Reproducible Construction

For each row in the candidate table:

```bash
git switch -c <clean-candidate-branch> origin/main
git merge --squash <canonical-source-branch>
git commit -m "fix: package gt 1.2 <subepic> candidate (gt-12-clean-subepic-pr-branches)"
git push fork <clean-candidate-branch>
```

The first attempt used `candidate/gt-1.2/*` local names. The fork pre-push guard rejected those names, so equivalent `polecat/ghoul/gt-1-2-candidate-*` refs were created at the same commits and pushed successfully.

## Research Pass Log

1. Read `bd show gt-12-clean-subepic-pr-branches` for scope, acceptance criteria, and packaging constraints.
2. Read `bd show gt-12-packaging-topology-audit` for canonical source refs and keep/drop rationale.
3. Read `bd show gt-12-review-package-subepics` for downstream package order and no-PR-before-review gate.
4. Read `bd show gt-12-convergence-cleanup` for parent release stabilization scope.
5. Compared `origin/main` and `origin/integration/gt-1-2-convergence-cleanup`; main remains the upstream PR base at `625bcf8a`, target is ahead at `a463ae52`.
6. Enumerated canonical `origin/integration/gt-1-2-*` and `origin/integration/gt-1.2/*` refs with tips and subjects.
7. Confirmed no existing `candidate/gt-1.2/*` or package candidate branches were present before construction.
8. Listed non-merge commits for the routing identity source leaf and confirmed raw WIP/autosave history exists.
9. Listed non-merge commits for the MR target/source source leaf and confirmed raw WIP/autosave history exists.
10. Listed non-merge commits for the polecat workstate source leaf and confirmed raw checkpoint history exists.
11. Listed non-merge commits for the recovery false-positive source leaf and confirmed raw WIP/autosave history exists.
12. Listed non-merge commits for the capacity/admission source leaf and confirmed raw autosave history exists.
13. Listed non-merge commits for the notification actionability source leaf and confirmed raw checkpoint/autosave history exists.
14. Compared changed-file stats for each selected source leaf against `origin/main` to verify each package has content.
15. Listed recent GitHub PR heads/bases/states and verified wrong-target artifacts and open main PRs should not be packaged directly in this pass.

## Pre-Implementation Review Log

1. Scope review: this pass prepares branch candidates and records evidence only; it must not open maintainer-facing PRs.
2. Source selection review: only topology-audited canonical content-bearing leaves are used; zero-delta canary/coordination refs are deferred or dropped.
3. History hygiene review: raw selected leaves contain WIP/autosave/checkpoint commits, so branches must be rebuilt as squashed candidates instead of reused directly.
4. Base review: upstream-main candidates must start at `origin/main`, while this manifest branch targets `integration/gt-1-2-convergence-cleanup` for merge-queue bookkeeping.
5. Branch policy review: pushed refs must use the allowed `polecat/*` namespace; no destructive branch deletion or force push is needed.

## Targeted Validation

- Each pushed clean candidate has exactly one commit ahead of `origin/main`.
- Each clean candidate's merge-base with `origin/main` is exactly `625bcf8a`.
- Each clean candidate tree matches its canonical source leaf exactly.
- `origin/integration/gt-1-2-release-candidate-and-canary-gate-canary` has `0 0` ahead/behind against `origin/main`, so it was explicitly deferred.
- `gh pr list` was used for evidence only; no maintainer-facing PRs were opened.

## Post-Implementation Review Log

1. Branch-count review: six content-bearing subepics have clean candidate branches; two zero-delta/evidence-only subepics have explicit defer/drop rationale.
2. Commit hygiene review: every clean candidate is a single non-WIP commit on `origin/main`.
3. Tree-equivalence review: each candidate tree matches the corresponding canonical source leaf, preserving package content while dropping noisy history.
4. Scope review: no wrong-target PR artifacts, stale old integration branches, fork duplicates, or open main PR heads were used as package sources.
5. PR-safety review: no PRs to `main` were opened; pushed branches are staged only for the downstream review/package gate.
