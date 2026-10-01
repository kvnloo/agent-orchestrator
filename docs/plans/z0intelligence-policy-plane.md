# z0intelligence policy + verification plane

Branch: `feat/z0intelligence-policy-plane`  
z0intelligence control thread: https://github.com/kvnloo/z0intelligence/issues/60

## Boundary

Agent Orchestrator remains the durable execution plane: session identity, worktrees,
controllers, harness lifecycle, permissions, delivery, SCM facts, and derived status.

z0intelligence is an optional policy/evidence plane. It may observe AO decisions,
emit receipts, recommend bounded alternatives, request verification, and learn
from outcomes. It cannot mutate AO canonical state or expand permissions.

## P0 implemented on this branch

Set `AO_Z0INTELLIGENCE_SHADOW_URL=http://127.0.0.1:11501` to enable the bridge.
Unset means no client is constructed and existing AO behavior is unchanged.

For each successful seed-session creation AO sends `ao.z0int.spawn.v1` with:
- stable trace id derived from the durable AO session id
- project + worker/orchestrator role
- raw task prompt only (no env, credentials, transcript, or private history)
- AO's already-resolved harness/model/mode/permission choice
- whether harness/model/mode came from an explicit caller constraint

The call is capped at 300 ms. Any timeout, malformed response, service error, or
unavailability is logged and ignored. P0 never applies the returned advice.

The client refuses non-loopback URLs so enabling the experimental bridge cannot
silently turn AO task text into arbitrary network egress.

## Why the hook is post-seed

The initial RFC suggested calling before `CreateSession`. That has no durable
idempotency identity: hashing task text would collapse two intentional identical
spawns, while a random id cannot survive a retry.

P0 therefore observes immediately after AO mints the seed session id. The trace
is `ao-spawn-<session-id>`, which is stable and naturally session-bound.

If a later promoted policy needs to influence pre-seed routing, first add a
first-class spawn idempotency key at the API/service boundary. Do not infer one
from prompt contents.

## P1a implemented on the outcome-ledger stack

AO outcomes are projected back to z0intelligence as `ao.z0int.outcome.v1`
using the same `ao-spawn-<session-id>` trace identity as the spawn decision.
Deterministic outcome ids distinguish durable termination from an early
`seed_deleted` rollback, so even failures that intentionally remove the seed
row remain joinable without AO persisting sidecar receipt state.

The wire intentionally separates the generic z0 Outcome from AO-specific
evidence. Durable termination alone does not claim execution completion or
verified success. A merged PR emits `pr_merged=true` with
`verification_source=ao-pr-merge`; otherwise failing CI can emit
`ci_failed=true`. Unknown facts stay absent rather than being guessed.

The bounded evidence envelope contains:
- AO session/project/role/harness/mode/model identity
- disposition (`terminated` or `seed_deleted`) + terminal flag + last normalized activity state
- up to 8 attributed PRs (newest first)
- draft/merged/closed, CI, review, mergeability, unresolved/external-review facts
- PR URL/number/head SHA for evidence joins

If PR evidence must be truncated, `scm_complete=false` makes the partial
snapshot explicit. The payload deliberately excludes review bodies, CI logs,
transcripts, task history, environment variables, credentials, and provider
payloads.

Outcome delivery is still shadow-only and fail-open with the same 300 ms budget.
AO re-reads canonical state after Kill/replacement termination and emits only
when `is_terminated` is durable. Failed-spawn paths that preserve a terminal
session also emit evidence. If an early rollback successfully deletes a seed
row, AO emits `seed_deleted` only after that delete succeeds; it claims no
execution completion and leaves SCM completeness false. `scm_complete=false`
otherwise distinguishes missing/truncated SCM projection from a legitimately
complete PR set.

## Promotion sequence

`off -> shadow -> assist -> selective authority`

Selective authority is allowed only for independently calibrated, bounded
decision families. AO permission/capability rules remain final.

Next: emit intermediate outcome revisions after durable lifecycle/SCM fact
changes, then measure join coverage, abstention, risk/coverage,
retries/corrections, verified outcomes, latency, and real token economics
before enabling assist mode.
