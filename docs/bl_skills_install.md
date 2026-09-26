---
title: "bl skills install"
slug: bl_skills_install
---
## bl skills install

Install or refresh Blaxel skills for coding agents

### Synopsis

Install Blaxel agent skills globally with a version-pinned, integrity-checked installer.
Requires Node.js 22.20.0 or later and npm. This explicit command runs even when
automatic installation is disabled with BL_INSTALL_SKILLS=false or in CI.

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

