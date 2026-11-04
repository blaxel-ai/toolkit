# Supported agent harnesses

Every blaxel-ai repository must work with these coding agents:

| Harness | Reads |
| --- | --- |
| Claude Code | `CLAUDE.md`, which imports `AGENTS.md` |
| Codex | `AGENTS.md` |
| OpenCode | `AGENTS.md` |
| Pi | `AGENTS.md` |
| Cursor | `AGENTS.md` |

`AGENTS.md` is the single source of instructions. Tool-specific files only import it, they never hold their own copy of the rules.
