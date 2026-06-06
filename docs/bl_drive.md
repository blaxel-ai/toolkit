---
title: "bl drive"
slug: bl_drive
---
## bl drive

Manage drives and drive mounts on sandboxes

### Synopsis

Manage drives and drive mounts on sandboxes.

Drive CRUD:
  bl drive list                       List all drives in the workspace
  bl drive get <name>                 Get details of a specific drive
  bl drive create                     Create a new drive
  bl drive delete <name>              Delete a drive

Sandbox mount operations:
  bl drive mount --sandbox <s> ...    Mount a drive to a running sandbox
  bl drive unmount --sandbox <s> ...  Unmount a drive from a running sandbox
  bl drive mounts --sandbox <s>       List drives mounted in a running sandbox

### Examples

```
  # List all drives
  bl drive list

  # Get details of a drive
  bl drive get my-drive

  # Create a new drive
  bl drive create --name my-drive --region us-pdx-1

  # Delete a drive
  bl drive delete my-drive

  # Mount a drive to a sandbox
  bl drive mount --sandbox my-sandbox --drive my-drive --mount-path /mnt/data

  # Unmount a drive from a sandbox
  bl drive unmount --sandbox my-sandbox --mount-path /mnt/data

  # List mounted drives in a sandbox
  bl drive mounts --sandbox my-sandbox
```

### Options

```
  -h, --help   help for drive
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
* [bl drive create](bl_drive_create.md)	 - Create a new drive
* [bl drive delete](bl_drive_delete.md)	 - Delete a drive
* [bl drive get](bl_drive_get.md)	 - Get details of a specific drive
* [bl drive list](bl_drive_list.md)	 - List all drives in the workspace
* [bl drive mount](bl_drive_mount.md)	 - Mount a drive to a sandbox
* [bl drive mounts](bl_drive_mounts.md)	 - List mounted drives in a sandbox
* [bl drive unmount](bl_drive_unmount.md)	 - Unmount a drive from a sandbox

