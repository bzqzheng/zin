# Architecture

Zin's planned v0 architecture has three separable layers:

1. Desktop shell and workspace UI
2. Local orchestration daemon
3. Agent runtime adapters

The desktop UI should be optimized for elegant living docs, comments, decision timelines, and human steering.

The daemon should be OS-agnostic and responsible for workspace state, task execution, isolated workdirs, runtime adapter invocation, event streaming, and local persistence.

Agent runtimes should be plug-and-use. Claude, Codex, OpenCode, Hermes, Kimi, Pi, and future tools should be adapters rather than hard-coded product assumptions.

## Early Technology Anchor

- Desktop: Tauri + React + TypeScript
- Living document editor: Tiptap / ProseMirror
- Orchestration daemon: Go
- Local persistence: SQLite first, PGlite still open for evaluation
- Execution isolation: per-task workdirs

