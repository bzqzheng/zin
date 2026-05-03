# Zin

Zin is an early-stage living workspace for human-agent collaboration.

The product thesis:

> Humans express intent, taste, strategy, and decisions. Agents help write, reason, execute, and remember.

Zin is not trying to replace agent CLIs, docs tools, chat tools, or project trackers. It is the orchestration layer where those tools can become part of one living workspace.

## Status

Pre-alpha scaffold. The public repo is intentionally minimal while the product direction and first implementation slice are being shaped.

## Initial Product Direction

Zin will prioritize:

- living docs for strategy, planning, and execution context
- decision timelines that preserve alternatives considered and outcomes reached
- comments, threads, and agent collaboration around documents
- tool-agnostic agent orchestration across Claude, Codex, OpenCode, Hermes, Kimi, Pi, and future runtimes
- local-first execution with a path to cloud-hosted collaboration

Kanban and issue state may exist as execution and audit tools, but they are not the center of the product.

## Planned Architecture

```text
apps/
  desktop/       Tauri + React desktop app
services/
  daemon/        Go orchestration daemon
packages/
  editor/        Tiptap/ProseMirror editor layer
  protocol/      Shared event and API types
docs/
  architecture.md
examples/
scripts/
```

## Regression QA

Run the Phase 3 smoke suite from the repo root:

```sh
./scripts/qa-smoke.sh
```

The script uses temporary test data, runs daemon contracts, desktop unit/integration tests, lint, build, and a headless Playwright flow against a mocked daemon API.

## License

Zin is currently source-available under a restrictive draft license. See `LICENSE`.

The license is intentionally conservative while the project is pre-alpha. It may change before public release.
