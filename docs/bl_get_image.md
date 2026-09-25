---
title: "bl get image"
slug: bl_get_image
---
## bl get image

List image summaries or image tags with cursor pagination

### Synopsis

List one page of image repository summaries, or one page of tags for a named image.
Use --cursor to continue a listing and --all to fetch every page. Empty pages
may still have a next cursor. Repository summaries retain total size, tag count,
status and last deployment time without downloading their tags.

Search uses a case-sensitive name prefix. Search results are ordered by name
ascending. Tag pages support name:asc and name:desc only.

--latest inspects every tag page and prints the most recently created tag reference.

```
bl get image [resourceType/imageName[:tag]] [flags]
```

### Examples

```
  bl get images --limit 100
  bl get images --q base --sort name:asc
  bl get image sandbox/base --limit 20
  bl get image sandbox/base --cursor CURSOR
  bl get image sandbox/base:v1
  bl get image sandbox/base --all
  bl get image sandbox/base --latest
```

### Options

```
      --all                       Fetch all pages instead of a single page
      --cursor string             Cursor from the previous page; keep the same search and sort
  -h, --help                      help for image
      --latest                    Return the most recent tag by creation time (reads all tag pages)
      --limit int                 Maximum items per page (1-100) (default 100)
      --q string                  Filter by a case-sensitive image or tag name prefix
      --sort string               Sort: name:asc, name:desc, createdAt:asc or createdAt:desc (tags: name only)
      --source-workspace string   Owner workspace of a shared image (named images only)
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

