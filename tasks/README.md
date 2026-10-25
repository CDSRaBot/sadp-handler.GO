# Development task artifacts

These files are repository artifacts for the development workflow described in
`docs/ru/DEVELOPMENT_GUIDE.md`. The normative task tracker is GitHub Issues;
these files preserve the local context, specifications, test scenarios,
deliverables, dependencies, and acceptance criteria used for implementation.

## Current plan

| Artifact | Issue | Status |
|---|---:|---|
| `phase-4-p7-configuration.md` | #35 | completed |
| `phase-5-m8-acceptance.md` | #37 | acceptance pending |
| `phase-5-m4-mutation-protocol.md` | #36 | audit completed; protocol contract blocked |
| `phase-5-verification-gate.md` | #38 | blocked by #36 and #37 |

Completed commits currently include:

- M7 locale validation: `860c200`
- M8 composition: `7bb23eb`
- M4 mutation seams and outcomes: `cd02e62`, `1b9d461`, `f3abe5c`
- CLI outcome rendering: `73c6945`

Current audit artifact:

- `docs/spec/procedures/logic/mutation_workflow_audit.md`
