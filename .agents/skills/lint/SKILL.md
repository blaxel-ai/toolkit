---
name: lint
description: Lint all changed Go files, fix every error, and confirm the build passes. Use when the user says "fix lint", "run lint", "lint", or when pre-commit hooks fail. Also use this proactively before committing code or when you notice linter warnings in build output.
---

## Step 1: Find changed files

```bash
git diff --name-only HEAD
git diff --name-only --cached
```

Determine which Go packages are affected so you know where to focus fixes.

## Step 2: Run linter

```bash
make lint
```

This runs `golangci-lint run`. Capture the full output -- you need the exact errors and file locations to fix them.

## Step 3: Fix every error

Common issues and how to fix them:
- **unused variables/imports** -- remove them
- **error return not checked** -- add `if err != nil` handling
- **ineffectual assignment** -- remove or use the variable
- **shadow variable** -- rename the inner variable
- **interface{} can be replaced by any** -- only fix in files you changed, not pre-existing ones

Do NOT run `golangci-lint run --fix` -- it only auto-fixes a handful of things and misses most issues. Read the errors and fix them manually in the code.

## Step 4: Re-run linter to confirm

After all fixes, re-run `make lint` to confirm zero errors.

If errors persist, fix them and re-run again. Do not stop until the linter passes cleanly.

## Step 5: Verify build

```bash
go build ./...
```

Linter fixes can sometimes break compilation (e.g., removing an import that was actually used elsewhere). This step catches that.

## Step 6: Report

Summarize what was fixed:
- Number of errors fixed
- What kinds of fixes were applied
- Confirmation that lint and build pass
