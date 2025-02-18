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
  -o, --output string      Output format. One of: pretty,yaml,json,table
  -u, --utc                Enable UTC timezone
  -v, --verbose            Enable verbose output
  -w, --workspace string   Specify the workspace name
```

### SEE ALSO

* [bl](bl.md)	 - Blaxel CLI is a command line tool to interact with Blaxel APIs.
* [bl delete agent](bl_delete_agent.md)	 - Delete a Agent
* [bl delete function](bl_delete_function.md)	 - Delete a Function
* [bl delete integrationconnection](bl_delete_integrationconnection.md)	 - Delete a IntegrationConnection
* [bl delete model](bl_delete_model.md)	 - Delete a Model
* [bl delete policy](bl_delete_policy.md)	 - Delete a Policy

