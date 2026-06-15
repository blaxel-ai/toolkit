# AGENTS.md

Guidance for AI agents working in the Blaxel toolkit repository.

## What this repo is

`github.com/blaxel-ai/toolkit` is the Blaxel CLI (`bl` / `blaxel`), a Go program
built on [Cobra](https://github.com/spf13/cobra). It wraps the Blaxel Go SDK
(`github.com/blaxel-ai/sdk-go`) and lets users log in, create/deploy agents, MCP
servers, sandboxes, jobs, manage drives, stream logs, and run resources.

Entry point: `main.go` → `cli.Execute` (`cli/root.go`) → `core.Execute`
(`cli/core/root.go`). Requires Go 1.25+.

## Commands (Makefile)

| Task | Command |
|------|---------|
| Dev build (installs `bl`/`blaxel` to `~/.local/bin`) | `make build-dev` |
| Release build (goreleaser) | `make build` |
| Unit tests | `make test` (`go test -count=1 ./...`) |
| Integration tests (needs `BL_API_KEY`, `BL_WORKSPACE`) | `make test-integration` |
| Lint | `make lint` (`golangci-lint run`) |
| Regenerate CLI docs | `make doc` |
| Regenerate SDK from controlplane | `make sdk-controlplane [branch=<x>]` |

Always run `make lint` and `go build ./...` before committing. Integration
tests hit the real platform — never run them against prod data; use a dev
workspace.

## Layout

```
main.go              # binary entry, version/commit ldflags, Sentry init
cli/                 # all command implementations (package cli)
  root.go            # cli.Execute wrapper
  core/              # shared: root command, config, auth, http, registry, doc gen
  register/          # SDK-operation → CLI command bridge
  auth/ chat/ connect/ deploy/ monitor/ server/   # subsystem packages
  *.go               # one file per top-level command (drive.go, get.go, ...)
  *_test.go          # unit tests live next to the code
docs/                # AUTO-GENERATED CLI reference (do not hand-edit; see below)
samples/             # example YAML resource configs
test/                # integration tests + install-script tests
contrib/             # zsh-blaxel-prompt plugin
definition.yml       # OpenAPI-derived spec driving generated SDK operations
```

## Command architecture

Each command file registers itself in an `init()` via the registry, then builds
its Cobra tree:

```go
func init() {
    core.RegisterCommand("drive", func() *cobra.Command { return DriveCmd() })
}

func DriveCmd() *cobra.Command {
    cmd := &cobra.Command{ Use: "drive", Short: "...", Long: "...", Example: "..." }
    cmd.AddCommand(DriveListCmd(), DriveGetCmd(), ...)
    return cmd
}
```

To add a command: create `cli/<name>.go`, register it in `init()`, build the
Cobra command, add subcommands, write a `*_test.go`, then run `make doc`.

## CLI documentation generation (important)

The `docs/` directory and the `blaxel-ai/docs` repo's CLI reference pages are
**auto-generated** from this repo by `bl docs` / `make doc`
(`cli/core/doc.go` → `cobra/doc.GenMarkdownTreeCustom`) and synced upstream.
Fix any CLI reference page **here, in the command's source** — edits to the
`docs` repo get overwritten on the next sync.

### Keep help text MDX-safe

Cobra writes a command's `Long` field **verbatim** under `### Synopsis`, with no
code fence. Mintlify renders these pages as MDX, which is strict about HTML/JSX.
Any pre-formatted content in `Long` (or `Short`) must be wrapped in a code fence
in the source, or the docs build breaks:

- Angle-bracket placeholders like `<name>` or `<s>` are parsed as HTML tags.
  `<s>` is a real strikethrough tag and silently breaks everything after it;
  `<name>` is consumed and disappears. This crashes the Mintlify preview build.
- Column alignment built with multiple spaces collapses outside a code block.

Convention (renders correctly in both the terminal `--help` and the docs): wrap
the block in a fence directly in the Go string. See `cli/get.go`
(`bl get mcp-hub` / `sandbox-hub`) and `cli/drive.go`:

```go
Long: `Short prose intro.

` + "```" + `
Pre-formatted listing with <placeholders> and aligned columns
` + "```",
```

`cmd.Example` is already fenced by Cobra, so examples are safe.

After changing any help text, regenerate and inspect before shipping:

```bash
go run . docs -o /tmp/bldocs && cat /tmp/bldocs/bl_<command>.md
```

## Code style

- Go conventions; format with `gofmt`. Run `make lint` before committing.
- Only fix `interface{} → any` (and similar nits) in files you actually changed,
  not pre-existing code in unrelated files.
- Keep tests next to the code as `*_test.go`. Add a unit test for new code paths.

## Agent skills

Repo-local skills live in `.agents/skills/`:
- `build-dev` — `make build-dev`
- `lint` — run `make lint`, fix every error, confirm build passes
