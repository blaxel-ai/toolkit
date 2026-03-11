---
name: build-dev
description: Build the CLI in development mode. Use when the user says "build", "build dev", "compile", "rebuild", or when you need to test local changes to the CLI binary. Also use this after writing code changes to verify they compile correctly, or when the user wants to try out their changes locally.
---

Run `make build-dev` from the repository root.

```bash
make build-dev
```

This compiles the `blaxel` binary with `go build`, injecting `version=dev` and the current git commit via `-ldflags`, then copies it to `~/.local/bin/blaxel` and `~/.local/bin/bl` so the user can immediately test with `bl` or `blaxel`.

If the build fails, read the error output carefully, fix the issue in the source code, and re-run until it succeeds.
