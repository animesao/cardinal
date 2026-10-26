# Work records

This directory is for multi-step work that needs a written goal, evidence, and a
handoff. It exists so a fresh session can recover state without re-reading the
whole repository.

It is not a roadmap, a release log, or a place for architecture prose.

## When to use it

Use a work record when at least one of these is true:

- the change spans several packages or several sessions;
- another agent or maintainer must continue the work;
- a decision needs an explicit scope and non-goals;
- the verification is partly blocked by the environment.

For a one-file fix, use the change protocol in the root
[`AGENTS.md`](../../AGENTS.md) and skip this directory.

## Layout

| Path | Purpose |
|---|---|
| [`templates/task.md`](templates/task.md) | Copy this before starting. State goal, scope, evidence, and verification. |
| [`templates/handoff.md`](templates/handoff.md) | Fill this in before stopping or handing off. Exact commands and results. |
| [`active/`](active/README.md) | Work in progress. Create one file per task, named after the branch or slug. |

## Workflow

1. Read the root [`AGENTS.md`](../../AGENTS.md) and the
   [`context map`](../ai/context-map.md).
2. Copy `templates/task.md` to `active/<slug>.md` and fill in the goal, scope,
   non-goals, and the smallest evidence that can prove the current behavior.
3. Check [`known-issues.md`](../status/known-issues.md) for existing drift before
   "fixing" documentation.
4. Do the work in small steps. Run the verification named in the task, not a
   broader command that only hides failures.
5. Copy `templates/handoff.md` to the same file or a sibling and record exact
   commands, results, and checks that could not run.
6. If a durable decision changed, write an ADR in
   [`docs/decisions/`](../decisions/). If a durable contradiction was found or
   fixed, update [`known-issues.md`](../status/known-issues.md).
7. When the work is done, either delete the temporary record or move it to
   `docs/plans/` if it is useful historical intent. Do not leave a completed
   task in `active/`.

## Rules

- Use the fact labels `Implemented`, `Partial`, `Planned`, `Deprecated`, and
  `Unknown` from the root [`AGENTS.md`](../../AGENTS.md).
- Point every non-obvious claim to a file, test, command, or commit.
- Never write secrets, tokens, private keys, or customer data into a work record.
- Do not claim a check passed unless it was run and the output was seen. Write
  "not run on this host" when the environment cannot run it.
- Keep records short. The source of truth is code and tests, not this directory.
