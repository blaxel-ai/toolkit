---
title: "bl delete"
slug: bl_delete
---
## bl delete

Delete resources from your workspace

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

  # Delete a sandbox preview
  bl delete sandbox my-sandbox preview my-preview

  # Delete a sandbox preview token
  bl delete sandbox my-sandbox preview my-preview token my-token

  # Delete from YAML file
  bl delete -f my-resource.yaml

  # Delete multiple resources from directory
  bl delete -f ./resources/ -R

  # Delete from stdin (useful in pipelines)
  cat resource.yaml | bl delete -f -

  # Safe deletion workflow
  bl get agent my-agent    # Review resource first
  bl delete agent my-agent # Delete after confirmation

  # --- Bulk deletion with jq filtering ---
  # WARNING: Bulk deletions are irreversible. Always preview first!

  # STEP 1: Preview what would be deleted (ALWAYS DO THIS FIRST)
  bl get jobs -o json | jq -r '.[] | select(.status == "DELETING") | .metadata.name'

  # STEP 2: After verifying the list, proceed with deletion
  bl delete jobs $(bl get jobs -o json | jq -r '.[] | select(.status == "DELETING") | .metadata.name')

  # More bulk deletion examples (always preview first):
  bl delete sandboxes $(bl get sandboxes -o json | jq -r '.[] | select(.status == "FAILED") | .metadata.name')
  bl delete agents $(bl get agents -o json | jq -r '.[] | select(.metadata.name | contains("test")) | .metadata.name')
  bl delete volumes $(bl get volumes -o json | jq -r '.[] | select(.metadata.labels.environment == "dev") | .metadata.name')
  bl delete sandboxes $(bl get sandboxes -o json | jq -r '.[] | select(.metadata.name | test("^temp-")) | .metadata.name')
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

* [bl](bl.md)	 - Blaxel CLI - manage and deploy AI agents, sandboxes, and resources
* [bl delete agent](bl_delete_agent.md)	 - Delete one or more agents
* [bl delete drive](bl_delete_drive.md)	 - Delete one or more drives
* [bl delete function](bl_delete_function.md)	 - Delete one or more functions
* [bl delete image](bl_delete_image.md)	 - Delete images or image tags
* [bl delete integrationconnection](bl_delete_integrationconnection.md)	 - Delete one or more integrationconnections
* [bl delete job](bl_delete_job.md)	 - Delete one or more jobs
* [bl delete model](bl_delete_model.md)	 - Delete one or more models
* [bl delete policy](bl_delete_policy.md)	 - Delete one or more policies
* [bl delete preview](bl_delete_preview.md)	 - Delete one or more previews
* [bl delete previewtoken](bl_delete_previewtoken.md)	 - Delete one or more previewtokens
* [bl delete sandbox](bl_delete_sandbox.md)	 - Delete one or more sandboxes
* [bl delete volume](bl_delete_volume.md)	 - Delete one or more volumes
* [bl delete volumetemplate](bl_delete_volumetemplate.md)	 - Delete one or more volumetemplates

