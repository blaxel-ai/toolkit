---
title: "bl delete"
slug: bl_delete
---
## bl delete

Delete a resource

```
bl delete [flags]
```

### Examples

```

bl delete -f ./my-resource.yaml
# Or using stdin
cat file.yaml | blaxel delete -f -
		
```

### Options

```
  -f, --filename string   containing the resource to delete.
  -h, --help              help for delete
  -R, --recursive         Process the directory used in -f, --filename recursively. Useful when you want to manage related manifests organized within the same directory.
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
* [bl delete agent](bl_delete_agent.md)	 - Delete agent
* [bl delete function](bl_delete_function.md)	 - Delete function
* [bl delete integrationconnection](bl_delete_integrationconnection.md)	 - Delete integrationconnection
* [bl delete job](bl_delete_job.md)	 - Delete job
* [bl delete model](bl_delete_model.md)	 - Delete model
* [bl delete policy](bl_delete_policy.md)	 - Delete policy
* [bl delete sandbox](bl_delete_sandbox.md)	 - Delete sandbox
* [bl delete volume](bl_delete_volume.md)	 - Delete volume

