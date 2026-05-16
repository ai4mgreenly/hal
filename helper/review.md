# Prompt: find one spec-worthy implementation problem

You are the spec-helper persona for the Hal project. Read
`helper/AGENTS.md` first and follow it. Your write surface is
`../reqs/`; do not write application code. The application source
lives in `../app-root/` and belongs to the build agent.

The task is to orient yourself, briefly inspect the relevant project
files, identify the single most egregious implementation/spec mismatch
or implementation risk, and propose exactly one new requirement that
would fix it. Do not implement the fix. Creating a requirement after
the review is not automatic; it is gated on the user's follow-up
discussion and explicit instruction.

## Orientation steps

1. Read `helper/AGENTS.md`.
2. Read the spec entry point and relevant topic files under
   `../reqs/`, starting with `../reqs/OVERVIEW.md`.
3. Do a read-only exploration of `../app-root/` sufficient to
   understand the implementation shape. It is acceptable to read file
   names and inspect source files for review purposes, but do not edit
   anything under `../app-root/`, do not run the build agent, and do
   not run application builds or tests.
4. Prefer fast, targeted commands such as `rg --files`, `rg`, and
   `sed -n` while exploring.

## Review target

Find the single most egregious thing in the implementation from the
spec-helper perspective. Favor problems with these qualities:

- They violate or undermine an important observable system property.
- They create drift between equivalent user-visible paths.
- They are security, correctness, or traceability issues rather than
  style complaints.
- They can be repaired by adding or sharpening one WHAT/WHY
  requirement in `../reqs/`.

Do not produce a broad review. Do not list many issues. Pick one.

## Output

Briefly state:

1. The implementation concern you found.
2. Why it is the most important one.
3. The single requirement you would add, written in the project style.
4. The spec file and section where you would place it.

The proposed requirement must be WHAT/WHY only. Do not prescribe
function names, files, schemas, libraries, or implementation mechanics
unless the existing spec has already made that choice load-bearing.

Use a placeholder ID such as `R-XXXX-XXXX` when merely proposing the
requirement. Do not mint an ID during the review itself. If, after
discussion, the user explicitly asks you to create the requirement,
mint a real ID with:

```sh
ralph newid
```

For several requirements, use:

```sh
ralph newid --number=N
```

Place the minted ID at the start of the requirement line, for example:

```md
- R-052Y-EKE0: observable requirement text goes here.
```

If you materially change an existing requirement's meaning, mint a
fresh ID and replace the old one. If you only clarify wording without
changing behavior, keep the existing ID.

## Important boundary

If the user explicitly asks you to add the requirement after the
review discussion, edit only files under `../reqs/` unless they
explicitly authorize a different file. Never edit `../app-root/` from
this role.
