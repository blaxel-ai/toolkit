---
title: "bl skills"
slug: bl_skills
---
## bl skills

Manage Blaxel skills for coding agents

### Synopsis

Manage the Blaxel agent skills installed for your coding agents.

Once skills are installed, ordinary bl commands and bl mcp check for a newer
verified skills bundle at most every six hours. Turn this off with
bl skills autoupdate off, or with BL_INSTALL_SKILLS=false; CI is skipped.

To uninstall, run bl skills autoupdate off so automatic updates and CLI
upgrades stop reinstalling them, then delete the blaxel-* folders in
~/.agents/skills and the matching links in each coding agent's skills folder
(for example ~/.claude/skills). The update state stays in ~/.blaxel/skills.

### Options

```
  -h, --help   help for skills
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

* [bl](bl.md)	 - Blaxel CLI - manage and deploy AI agents, sandboxes, and resources
* [bl skills autoupdate](bl_skills_autoupdate.md)	 - Save this machine's automatic skills update preference
* [bl skills install](bl_skills_install.md)	 - Install or refresh Blaxel skills for coding agents
* [bl skills status](bl_skills_status.md)	 - Show skills revisions, update checks, skips and failures
* [bl skills update](bl_skills_update.md)	 - Refresh skills from the compatible verified release bundle

