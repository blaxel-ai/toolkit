# AGENTS.md

Guidance for AI agents working in the Blaxel toolkit (CLI) repository.

## CLI documentation generation

The `docs` repo (`blaxel-ai/docs`) does not hand-write the CLI reference pages.
They are auto-generated from this repo by `bl docs`
(`cli/core/doc.go` → `cobra/doc.GenMarkdownTreeCustom`) and synced upstream.
Any fix to a CLI reference page must be made **here**, in the command's source,
not in the `docs` repo (edits there get overwritten on the next sync).

### Mintlify (MDX) safe `Long` / `Short` / `Example` text

Cobra writes a command's `Long` field **verbatim** under `### Synopsis`, with no
code fence. Mintlify parses these pages as MDX, which is strict about HTML/JSX.
So any pre-formatted content in `Long` (or `Short`) must be wrapped in a code
fence in the source, otherwise the docs build breaks:

- Angle-bracket placeholders like `<name>` or `<s>` are parsed as HTML tags.
  `<s>` is a real strikethrough tag and silently breaks everything after it;
  `<name>` is consumed and disappears. This crashes the Mintlify preview build.
- Column alignment built with multiple spaces collapses outside a code block.

Convention used in this repo: wrap such blocks in a fence directly in the Go
string so it renders correctly in both the terminal `--help` and the generated
docs. See `cli/get.go` (`bl get mcp-hub` / `sandbox-hub`) and `cli/drive.go`:

```go
Long: `Short prose intro.

` + "```" + `
Pre-formatted listing with <placeholders> and aligned columns
` + "```",
```

`cmd.Example` is already fenced by Cobra, so examples are safe.

After changing any command help text, regenerate and inspect the output before
shipping:

```bash
go run . docs -o /tmp/bldocs && cat /tmp/bldocs/bl_<command>.md
```
