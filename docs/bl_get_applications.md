---
title: "bl get applications"
slug: bl_get_applications
---
## bl get applications

List all applications or get details of a specific one

```
bl get applications [flags]
```

### Options

```
      --all             Fetch all pages (may be slow for large collections)
      --cursor string   Cursor from a previous page to fetch the next page of results
  -h, --help            help for applications
      --limit int       Maximum number of items to return (auto-paginates when above 200) (default 200)
```

### Options inherited from parent commands

```
  -o, --output string          Output format. One of: pretty,yaml,json,table
      --skip-version-warning   Skip version warning
  -u, --utc                    Enable UTC timezone
  -v, --verbose                Enable verbose output
      --watch                  After listing/getting the requested object, watch for changes.
  -w, --workspace string       Specify the workspace name
```

### SEE ALSO

* [bl get](bl_get.md)	 - List or retrieve Blaxel resources in your workspace

