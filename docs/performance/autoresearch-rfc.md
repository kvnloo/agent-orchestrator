# AO end-to-end performance autoresearch (fork-only proposal)

**Status:** Research scaffold; not an upstream RFC, benchmark result, or claim of live AO improvement. Seek maintainer scope approval before code contributions per [CONTRIBUTING](https://github.com/OrchestratorInc/agent-orchestrator/blob/main/CONTRIBUTING.md).

## Objective
Reduce click-to-useful-response, first frame, typing/echo latency and UI stalls on the actual Electron + Go daemon + PTY + network + React pipeline. Maximize impact on the user's **interaction**, not an unrelated local microbenchmark. Preserve correctness, accessibility, replay, selection/copy, input ordering, approvals and persistence.

## Existing contributors/work to build upon
- AgentWrapper: [heavy terminal throughput #6232](https://github.com/OrchestratorInc/agent-orchestrator/pull/6232), [WebGL/burst batching #6465](https://github.com/OrchestratorInc/agent-orchestrator/pull/6465), [Ghostty WASM exploration #6238](https://github.com/OrchestratorInc/agent-orchestrator/issues/6238).
- ronishrohan: [long history #6129](https://github.com/OrchestratorInc/agent-orchestrator/pull/6129), [streaming memo #6315](https://github.com/OrchestratorInc/agent-orchestrator/pull/6315). Pulkit7070: [chat responsiveness #4947](https://github.com/OrchestratorInc/agent-orchestrator/pull/4947).
- Vaibhaav-Tiwari: [file-viewer manifest/cache #6145](https://github.com/OrchestratorInc/agent-orchestrator/pull/6145).
- illegalcall: [journey instrumentation #5868](https://github.com/OrchestratorInc/agent-orchestrator/pull/5868) and [hover prefetch #5980](https://github.com/OrchestratorInc/agent-orchestrator/pull/5980).
- Pyasma: [process diagnostics #5837](https://github.com/OrchestratorInc/agent-orchestrator/pull/5837).
- Harshit Singh Bhandari: [CI performance evidence #5116](https://github.com/OrchestratorInc/agent-orchestrator/issues/5116).

## Experiments to investigate, *not* parallel competing patches
1. Click → genuinely usable composer/terminal, including fallback elapsed time: [#5361](https://github.com/OrchestratorInc/agent-orchestrator/issues/5361), [#5890](https://github.com/OrchestratorInc/agent-orchestrator/issues/5890), [#5868](https://github.com/OrchestratorInc/agent-orchestrator/pull/5868).
2. Writer and PTY long-running stability: [#6219](https://github.com/OrchestratorInc/agent-orchestrator/issues/6219), [#6259](https://github.com/OrchestratorInc/agent-orchestrator/issues/6259). Reuse [existing fork-only regression #44](https://github.com/kvnloo/agent-orchestrator/pull/44).
3. Handshake/reconnect idle rendering storm: [#6466](https://github.com/OrchestratorInc/agent-orchestrator/issues/6466).
4. Huge Changes view, correct diff semantics: [#6391](https://github.com/OrchestratorInc/agent-orchestrator/issues/6391).
5. Residual terminal paint time, only if measured and parity feasible: [#6238](https://github.com/OrchestratorInc/agent-orchestrator/issues/6238).

## Loop
- Check upstream latest head and ownership. Establish a reproducible, **immutable** workload and exact measurement boundaries on **scratch** app/data.
- Add RED production-path test, smallest change, GREEN, functional regression test.
- Re-run baseline and candidate on **the same machine** in balanced ABBA/BAAB blocks. Exclude model inference noise when measuring renderer; include actual model inference only when explicitly measuring full user journey. Record host, OS, GPU, display, main SHA, candidate SHA and fixture hash.
- Compare p50/p95/p99, time-to-first-useful-paint, dropped frames/long tasks, CPU/RSS and guardrails. Reject apparent gains where p95 regresses or correctness changed. A single significant result is **candidate**, not proved.
- Have an independent reviewer replay the result and inspect for benchmark overfit and accessibility/selection/input regressions. Keep only scoped, independently reproducible improvements; retain rejected trials/receipts.
- Keep experiment code and receipts in fork-only branches; do not automatically post, comment or open upstream PRs.

## Existing tests and docs
See [file-viewer performance](https://github.com/OrchestratorInc/agent-orchestrator/blob/main/docs/performance/file-viewer.md), [chat responsiveness](https://github.com/OrchestratorInc/agent-orchestrator/blob/main/docs/performance/chat-responsiveness/README.md), and [renderer E2E](https://github.com/OrchestratorInc/agent-orchestrator/tree/main/frontend/e2e). Reuse them before adding another framework.

## Guardrails
No arbitrary local benchmark wins, no single-frame placebo, no silent deletion of completed offscreen content without search/copy/a11y parity, no cross-host fixture comparisons, no real user datasets, no autonomous upstream posting or merges. Coordinate first with the issue owners and [AO Discord](https://discord.com/invite/UZv7JjxbwG).