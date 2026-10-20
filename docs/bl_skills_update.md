---
title: "bl skills update"
slug: bl_skills_update
---
## bl skills update

Refresh skills from the compatible verified release bundle

### Synopsis

Check the trusted Blaxel skills manifest and install its compatible checksum-verified bundle.
Modified folders, manager links and plugin copies are preserved. This command
ignores automatic update opt-outs. Restart your coding agent to load changes.
Until the first bundle is published, use bl skills install for initial installation.

```
bl skills update [flags]
```

### Options

```
  -h, --help   help for update
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

