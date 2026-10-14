---
title: "bl custom-domain"
slug: bl_custom-domain
---
## bl custom-domain

Manage custom domains

### Synopsis

Manage the custom domains of the current workspace.

A custom domain serves previews or applications under your own hostname. After
creating it, publish the DNS records shown by the records command, then verify it.

```
  bl custom-domain list                      List custom domains
  bl custom-domain get <name>                Get a custom domain
  bl custom-domain create <name> --type ...  Create a custom domain
  bl custom-domain records <name>            Show the DNS records to publish
  bl custom-domain verify <name>             Check the DNS records and activate the domain
  bl custom-domain delete <name>             Delete a custom domain
```

### Examples

```
  # Create an application domain with a www alternative name
  bl custom-domain create example.com --type applications --san www.example.com

  # Show the DNS records to publish, then verify
  bl custom-domain records example.com
  bl custom-domain verify example.com
```

### Options

```
  -h, --help   help for custom-domain
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
* [bl custom-domain create](bl_custom-domain_create.md)	 - Create a custom domain
* [bl custom-domain delete](bl_custom-domain_delete.md)	 - Delete a custom domain
* [bl custom-domain get](bl_custom-domain_get.md)	 - Get a custom domain
* [bl custom-domain list](bl_custom-domain_list.md)	 - List custom domains
* [bl custom-domain records](bl_custom-domain_records.md)	 - Show the DNS records to publish for a custom domain
* [bl custom-domain verify](bl_custom-domain_verify.md)	 - Check the DNS records of a custom domain and activate it

