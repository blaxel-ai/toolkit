---
title: "bl drive list"
slug: bl_drive_list
---
## bl drive list

List all drives in the workspace

### Synopsis

List all drives in the current workspace.

```
bl drive list [flags]
```

### Examples

```
  # List all drives
  bl drive list

  # List drives in JSON format
  bl drive list -o json

  # Fetch the next page
  bl drive list --cursor <cursor>

  # Fetch every drive
  bl drive list --all
```

### Options

```
      --all             Fetch all pages (may be slow for large collections)
      --cursor string   Cursor from a previous page to fetch the next page of results
  -h, --help            help for list
      --limit int       Maximum number of items to return (max 200) (default 200)
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

* [bl drive](bl_drive.md)	 - Manage drives and drive mounts on sandboxes

