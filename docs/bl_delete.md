---
title: "bl delete"
slug: bl_delete
---
## bl delete

Delete a resource

### Synopsis

Delete Blaxel resources from your workspace.

WARNING: Deletion is permanent and cannot be undone. Resources are immediately
deactivated and removed along with their configurations.

Two deletion modes:
1. By name: Use subcommands like 'bl delete agent my-agent'
2. By file: Use 'bl delete -f resource.yaml' for declarative management

What Happens:
- Resource is immediately stopped and deactivated
- Configuration and metadata are removed
- Associated logs and metrics may be retained (check workspace policy)
- Data volumes are NOT automatically deleted (use 'bl delete volume')

Before Deleting:
- Backup any important configuration or data
- Check dependencies (other resources using this one)
- Consider stopping instead of deleting for temporary disablement

Note: Deleting an agent/job stops it immediately but may not delete associated
storage volumes. Use 'bl get volumes' to see persistent storage and delete
separately if needed.

```
bl delete [flags]
```

### Examples

```
  # Delete by name (using subcommands)
  bl delete agent my-agent
  bl delete job my-job
  bl delete sandbox my-sandbox

  # Delete multiple resources by name
  bl delete volume vol1 vol2 vol3
  bl delete agent agent1 agent2

  # Delete from YAML file
  bl delete -f my-resource.yaml

  # Delete multiple resources from directory
  bl delete -f ./resources/ -R

  # Delete from stdin (useful in pipelines)
  cat resource.yaml | bl delete -f -

  # Safe deletion workflow
  bl get agent my-agent    # Review resource first
  bl delete agent my-agent # Delete after confirmation
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
* [bl delete image](bl_delete_image.md)	 - Delete images or image tags
* [bl delete integrationconnection](bl_delete_integrationconnection.md)	 - Delete integrationconnection
* [bl delete job](bl_delete_job.md)	 - Delete job
* [bl delete model](bl_delete_model.md)	 - Delete model
* [bl delete policy](bl_delete_policy.md)	 - Delete policy
* [bl delete sandbox](bl_delete_sandbox.md)	 - Delete sandbox
* [bl delete volume](bl_delete_volume.md)	 - Delete volume
* [bl delete volumetemplate](bl_delete_volumetemplate.md)	 - Delete volumetemplate

