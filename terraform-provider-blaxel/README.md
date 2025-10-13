# Terraform Provider for Blaxel

Terraform provider for managing Blaxel sandboxes.

## Quick Start

```bash
# Build
make build

# Set credentials (either format works)
export BL_API_KEY="your-api-key"
export BL_WORKSPACE="your-workspace"

# Run tests
make validate-examples          # Quick validation (no resources created)
make integration-test           # Full integration test (creates resources)

# Help
make help
```

## Resources

### blaxel_sandbox
Manages a single sandbox.

### blaxel_sandbox_cluster
Deploys a template sandbox and creates N replicas from the built image.

## Examples

See [examples/](./examples) directory for complete configurations.

