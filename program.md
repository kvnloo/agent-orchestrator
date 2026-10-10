# AO UX latency autoresearch — agent instruction

You are a skeptical performance researcher. Operate only in fork branches and isolated worktrees. **Never publish upstream comments/issues/PRs or merge without a human-approved scope.** Follow current `AGENTS.md`, `CONTRIBUTING.md`, `docs/STATUS.md`, and the fork research note `docs/performance/autoresearch-rfc.md`. Credit the original authors of ideas, analysis, fixes and tests.

## Objective
Optimize the user's *end-to-end interaction*: click → first usable composer; received text → visible painted reply; key → echo; session switch → first frame; Changes click → actionable diff. Prioritize p95/p99, responsiveness under concurrent sessions and correctness. Do not maximize an irrelevant microbenchmark.

## First check merged work before starting
- [#5626](https://github.com/OrchestratorInc/agent-orchestrator/pull/5626) already accelerated Chat startup. **Don't recreate its speculative worktree or background spawn.** Profile remaining *current-head* cold/warm journeys instead.
- [#6232](https://github.com/OrchestratorInc/agent-orchestrator/pull/6232) and [#6465](https://github.com/OrchestratorInc/agent-orchestrator/pull/6465) already improved PTY throughput, GPU context reuse and terminal batching.
- [#5632](https://github.com/OrchestratorInc/agent-orchestrator/pull/5632) already improved Go CI; preserve race test execution rather than just making checks green.
- [#6260](https://github.com/OrchestratorInc/agent-orchestrator/pull/6260) is an **active open** fix for the PTY ring leak in [#6259](https://github.com/OrchestratorInc/agent-orchestrator/issues/6259), owned by priyanshujaiz; review it independently, don't compete.
- [#5868](https://github.com/OrchestratorInc/agent-orchestrator/pull/5868), [#5766](https://github.com/OrchestratorInc/agent-orchestrator/pull/5766), [#5176](https://github.com/OrchestratorInc/agent-orchestrator/pull/5176), [#6198](https://github.com/OrchestratorInc/agent-orchestrator/pull/6198) are existing active or blocked efforts. Preserve their owners.

## Experiment loop
1. Fetch exact current upstream main and related issues/PR review. Check overlap before selecting a hypothesis.
2. Choose one scene, fixed fixtures and the exact latency boundary. Build/reuse a real app instrumentation adapter. Record initial baseline including cold/warm and p50/p95/p99 as justified by sample sizes.
3. Add a **RED production-path test** demonstrating the bottleneck or invariant; minimize scope to one owner; implement change; get GREEN.
4. Benchmark isolated baseline and candidate on the same machine in serial balanced ABBA / BAAB trials with identical data, warmed state and display/GPU conditions. Save exact SHAs, scripts, hardware/software fingerprint and fixture checksum.
5. Reject gains bought through input loss, stale data, dropped frames becoming invisible, broken approvals, truncated PTY replay, disabled search/selection/screen readers, or p95/p99 regressions. Include checks for each relevant invariant. Use an independent critic.
6. Keep/revert *only inside the disposable experiment worktree*. Append a receipt for accepted, rejected and inconclusive experiments; reproduce winners independently before recommending upstream.
7. Rotate bottleneck lanes: (A) task-create journey, (B) multiagent CDC/write stalls, (C) terminal input and attach, (D) long chat, (E) Files/Changes, (F) CI feedback. Parallelize **analysis** across Hermes, Codex and Claude; serialize physical CPU/GPU benchmarks to prevent interference.

## Stop
Stop at the agreed time/compute budget, failed measurement validity, stale branch, or ownership dispute. Produce a short report identifying actual speedups, rejected ideas, counterexamples, provenance, tests and original contributor credit. Never call a mocked Playwright or synthetic CLI measurement full packaged-Electron end-to-end. No hypothetical speedup may be stated as real.

## Reusable judge
The companion self-contained measurement judge and mock tests live in the downloadable `ao-autoresearch-v0.zip` artifact from the parent chat; it is **not yet included in this branch**. Place its `scripts/perf-autoresearch/` directory into the fork worktree and run `node --test scripts/perf-autoresearch/run.test.mjs`. Replace the mock benchmark adapter with actual AO app instrumentation before making product performance claims.