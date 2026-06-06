---
title: "bl drive mount"
slug: bl_drive_mount
---
## bl drive mount

Mount a drive to a sandbox

### Synopsis

Mount or re-mount a drive to a sandbox environment.

This command attaches an agent drive to a local path inside the sandbox using
the blfs filesystem. It can be used as a recovery tool when mounts are lost.

```
bl drive mount [flags]
```

### Examples

```
  # Mount a drive with default settings
  bl drive mount --sandbox my-sandbox --drive my-drive --mount-path /mnt/data

  # Mount a subdirectory of the drive
  bl drive mount --sandbox my-sandbox --drive my-drive --mount-path /mnt/data --drive-path /subdir

  # Mount as read-only
  bl drive mount --sandbox my-sandbox --drive my-drive --mount-path /mnt/data --read-only

  # Mount with UID/GID mapping
  bl drive mount --sandbox my-sandbox --drive my-drive --mount-path /mnt/data --uid-map 1000 --gid-map 1000
```

### Options

```
      --drive string        Name of the drive to mount
      --drive-path string   Subdirectory within the drive to mount (optional, defaults to /)
      --gid-map string      Local GID to map (filer GID is always 0)
  -h, --help                help for mount
      --mount-path string   Local path inside the sandbox to mount the drive
      --read-only           Mount the drive as read-only
      --sandbox string      Name of the sandbox
      --uid-map string      Local UID to map (filer UID is always 0)
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

