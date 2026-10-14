---
title: "bl custom-domain create"
slug: bl_custom-domain_create
---
## bl custom-domain create

Create a custom domain

### Synopsis

Create a custom domain in the current workspace.

--type previews serves sandbox previews under the domain (use a wildcard name
such as *.preview.example.com); --type applications serves applications.

--validation-method chooses how the TLS certificate is validated. dns (the API
default when the flag is omitted) asks you to publish certificate validation
records. http needs no validation record once the domain points to Blaxel; it
is only available for applications domains without wildcard names. The method
is set at creation: later updates keep the existing method.

```
bl custom-domain create <name> [flags]
```

### Examples

```
  # Application domain on an apex with www, DNS validation (default)
  bl custom-domain create example.com --type applications --san www.example.com

  # Application domain validated over HTTP
  bl custom-domain create app.example.com --type applications --validation-method http

  # Wildcard previews domain
  bl custom-domain create "*.preview.example.com" --type previews
```

### Options

```
  -h, --help                       help for create
      --san strings                Subject alternative name to add to the certificate (repeatable, applications only)
      --type string                Domain type: previews or applications
      --validation-method string   Certificate validation method: dns or http (API default: dns)
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

