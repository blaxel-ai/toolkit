---
title: "bl run"
slug: bl_run
---
## bl run

Run a resource on blaxel

```
bl run resource-type resource-name [flags]
```

### Examples

```
bl run agent my-agent --data '{"inputs": "Hello, world!"}'
bl run model my-model --data '{"inputs": "Hello, world!"}'
bl run job my-job --file myjob.json
```

### Options

```
  -d, --data string          JSON body data for the inference request
      --debug                Debug mode
  -f, --file string          Input from a file
      --header stringArray   Request headers in 'Key: Value' format. Can be specified multiple times
  -h, --help                 help for run
      --local                Run locally
      --method string        HTTP method for the inference request (default "POST")
      --params strings       Query params sent to the inference request
      --path string          path for the inference request
      --upload-file string   This transfers the specified local file to the remote URL
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

* [bl](bl.md)	 - Blaxel CLI is a command line tool to interact with Blaxel APIs.

