# Refactor iteration prompt

You are running one non-interactive implementation iteration for this project.
Read this file first, then read `../NEXT.md`, then perform exactly the single
action described there.

## Project layout

- `../app-root/` is the application code root and the default write surface.
  Make code, test, fixture, and build-support changes there as needed to
  complete the single action in `../NEXT.md`.
- `../reqs/` contains the existing project specifications. Read these files
  freely to understand the required behavior, but do not create, modify,
  rename, or delete anything under `../reqs/`.
- `../NEXT.md` describes the next single action to take. It may override the
  code root or narrow the write surface for that one iteration.
- `../helper/` contains operator and helper prompts. Do not modify helper files
  during a refactor iteration.
- `../app-root/.ralph/` is Ralph iteration state, not refactor-iteration input
  or output. Do not read, create, modify, chmod, rename, delete, validate, or
  otherwise depend on anything under `.ralph/` while running this prompt.

## Operating rules

- Run non-interactively. Do not ask the user questions or wait for
  clarification.
- Use your best judgment when details are missing. Choose the smallest
  reasonable interpretation that preserves the specs, project conventions, and
  the scope stated in `../NEXT.md`.
- Perform exactly the single action described in `../NEXT.md`. Do not continue
  into adjacent cleanup, future transformations, or speculative improvements
  unless they are necessary to complete that action.
- Treat `../reqs/` as read-only specification input.
- Prefer existing project patterns, tools, style, and tests over introducing
  new conventions.
- Preserve unrelated user changes. Do not revert or overwrite work outside the
  action's scope.
- Run focused verification when practical. If `../NEXT.md` names specific
  verification commands, prefer those.
- Do not perform Ralph ledger checks during refactor verification. If a broad
  command such as `go test ./...` fails only because a test reads
  `.ralph/requirements-verified.jsonl`, treat that as out-of-scope local Ralph
  state and continue with focused verification for the refactor action.

## Completion

When the action is complete, append a short result note to `../NEXT.md`. This is
the only permitted write outside the active code root. The note should include:

- a brief summary of the work completed
- the files changed
- verification performed, including commands and outcomes
- blockers, issues, or follow-up risks discovered

After appending the result note, commit all changes made during the iteration.
The commit is the final step. After committing, stop. Do not begin another
action.

If the result is imperfect, still stop after the single action, result note,
and commit, and report the state clearly. Future iterations will handle
dissatisfaction by refining `../NEXT.md`, this prompt, or the specifications
in fail-forward mode.
