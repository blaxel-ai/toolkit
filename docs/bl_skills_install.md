---
title: "bl skills install"
slug: bl_skills_install
---
## bl skills install

Install or refresh Blaxel skills for coding agents

### Synopsis

Install the latest Blaxel agent skills globally from github.com/blaxel-ai/agent-skills.
Skills go to ~/.agents/skills and to the coding agents detected on this machine
(Claude Code, Codex, Cursor, ...). Nothing else needs to be installed first.
This explicit command runs even when automatic installation is disabled with
BL_INSTALL_SKILLS=false or in CI.
Existing externally managed skill links are kept, not refreshed. Same-name
skills in nested folders are reused automatically. Conflicting copies are
backed up outside the agents' skills folders and replaced with links.

```
bl skills install [flags]
```

### Options

```
  -h, --help   help for install
```

### Options inherited from parent commands

```
  -o, --output string          Output format. One of: pretty,yaml,json,table
      --skip-version-warning   Skip version warning
  -u, --utc                    Enable UTC timezone
  -v, --verbose                Enable verbose output
  -w, --workspace string       Specify the workspace name
```

### SEE ALSO

* [bl skills](bl_skills.md)	 - Manage Blaxel skills for coding agents

