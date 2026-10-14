---
title: "bl custom-domain records"
slug: bl_custom-domain_records
---
## bl custom-domain records

Show the DNS records to publish for a custom domain

### Synopsis

Show the DNS records to publish at your DNS provider for a custom domain.

Routing records point the domain to Blaxel. An apex name (example.com) cannot
carry a CNAME, so its routing record is shown as ALIAS/ANAME: use your
provider's ALIAS, ANAME or CNAME flattening record. Certificate validation
records are only needed with the dns validation method.

```
bl custom-domain records <name> [flags]
```

### Examples

```
  bl custom-domain records example.com
  bl custom-domain records example.com -o json
```

### Options

```
  -h, --help   help for records
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

* [bl custom-domain](bl_custom-domain.md)	 - Manage custom domains

