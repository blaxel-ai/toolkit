---
title: "bl drive create"
slug: bl_drive_create
---
## bl drive create

Create a new drive

### Synopsis

Create a new drive in the current workspace.

```
bl drive create [flags]
```

### Examples

```
  # Create a drive in a specific region
  bl drive create --name my-drive --region us-pdx-1

  # Create a drive with a size limit (in GB)
  bl drive create --name my-drive --region us-pdx-1 --size 10
```

### Options

```
  -h, --help            help for create
      --name string     Name of the drive
      --region string   Deployment region (e.g., us-pdx-1, eu-lon-1)
      --size int        Size limit in GB (optional, 0 for unlimited)
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

