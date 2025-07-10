# OKRA Registry Approach

The OKRA registry system provides a flexible, distributed model for storing and retrieving services, types, and host runtimes. This document describes the registry architecture, storage options, and access patterns.

## Overview

OKRA registries are modular storage systems that can host multiple types of artifacts:

- **Services** (`.pkg` bundles) - Compiled WASM services with their metadata
- **Types** - Shared type definitions and event schemas
- **Hosts** - Runtime binaries with specific Host API implementations
- **Hybrid** - Registries that combine multiple artifact types

## Registry Configuration

Registries are configured in `~/.okra/registries.yml`:

```yaml
registries:
  - name: local
    type: [service, type]
    path: file:///Users/me/.okra/local-registry
    
  - name: company
    type: [service, type, host]
    path: s3://company-okra-registry
    region: us-east-1
    
  - name: okra-official
    type: host
    path: https://registry.okra.io
```

Clusters are separately configured in `~/.okra/clusters.yml` for deployment targets.

## Registry Types

### Service Registry
- Stores `.pkg` files containing compiled WASM services
- Each service has a unique FQN (Fully Qualified Name)
- Includes metadata like `okra.json` and `service.description.json`

### Type Registry
- Stores globally shared type definitions
- Enables cross-service type reuse
- Supports event schema definitions

### Host Registry
- Distributes OKRA host runtime binaries
- Each host includes specific Host API implementations
- Enables `okra dev` and `okra serve` to download and use appropriate hosts
- Selected based on `okra.json` `host` or `hostVersion` fields

## Storage Structure

### Primary Layout (FQN-based)

All registry entries are organized by Fully Qualified Name (FQN) following a path-based structure:

```
/com/studiomvp/auth/UserService/
  ├── 1.0.0/
  │   ├── service.pkg              # Compiled WASM bundle
  │   ├── okra.json               # Service metadata
  │   └── service.description.json # Generated service descriptor
  ├── 1.2.3/
  │   ├── service.pkg
  │   ├── okra.json
  │   └── service.description.json
  └── latest -> 1.2.3/            # Symlink to latest version

/com/studiomvp/shared/Email/     # Type definition
  ├── 1.0.0/
  │   └── type.proto
  └── latest -> 1.0.0/

/com/okra/hosts/default/         # Host runtime
  ├── 1.0.0/
  │   ├── okra-host-darwin-arm64
  │   ├── okra-host-darwin-amd64
  │   ├── okra-host-linux-amd64
  │   └── host.json              # Host capabilities manifest
  └── latest -> 1.0.0/
```

### Change Log Structure

To support efficient synchronization and cache invalidation, registries maintain a change log:

```
/updates/2025/07/09/
  ├── com.studiomvp.auth.UserService.update.json
  ├── com.studiomvp.shared.Email.update.json
  └── com.okra.hosts.default.update.json
```

Each update file contains:

```json
{
  "fqn": "com.studiomvp.auth.UserService",
  "version": "1.2.3",
  "type": "service",
  "updatedAt": "2025-07-09T03:20:41Z",
  "summary": "Added GetUserByToken RPC method",
  "previousVersion": "1.2.2"
}
```

This structure enables:
- Efficient incremental syncing via date-based prefix queries
- Cache invalidation notifications
- Audit trails for registry changes
- Minimal bandwidth for update checks

## Storage Backends

### Local Disk
- Path: `file:///path/to/registry`
- Ideal for monorepos and air-gapped environments
- Zero latency for local development

### S3-Compatible Object Storage
- Path: `s3://bucket-name/prefix`
- Supports AWS S3, MinIO, GCS, etc.
- Efficient for team/organization scale
- Leverages native list operations for change detection

### HTTP/HTTPS (Read-only)
- Path: `https://registry.example.com`
- For public or authenticated registries
- Future: Will support registry API protocol

## Registry Operations

### Publishing

```bash
# Publish a service
okra publish --registry=company

# Publish with specific version
okra publish --version=1.2.3 --registry=local

# Publish a host runtime
okra publish:host --registry=okra-official
```

### Resolving Dependencies

The registry resolution order:
1. Local workspace (`workspace:*` dependencies)
2. Configured registries in order from `~/.okra/registries.yml`
3. Default public registry (if configured)

### Caching

- Local cache at `~/.okra/cache/`
- Organized by registry name and FQN
- Uses change log for efficient invalidation
- Configurable TTL per registry

## Versioning and FQNs

### Service FQNs

Services use a hierarchical naming scheme with embedded version:

```
namespace.service.version
```

Examples:
- `com.studiomvp.auth.UserService.v1`
- `com.acme.billing.PaymentProcessor.v2`

### Version Resolution

1. **IDL `@okra` directive** provides namespace and optional version
2. **Version inference**: If version not specified in `@okra`, uses major version from `okra.json`
   - `okra.json` version `1.2.3` → IDL version `v1`
   - `okra.json` version `2.0.0` → IDL version `v2`
3. **Full semantic version** stored in registry (e.g., `1.2.3`)

### Version Compatibility

- Minor/patch updates are backward compatible
- Major version changes indicate breaking changes
- Services can depend on version ranges: `"^1.2.0"`, `"~1.2.3"`

## Security and Access Control

### Authentication
- Local registries: File system permissions
- S3 registries: IAM roles or credentials
- HTTP registries: Bearer tokens or API keys

### Integrity
- Each artifact includes SHA-256 checksum
- Checksums verified on download
- Optional signing for enterprise deployments

## Best Practices

1. **Use semantic versioning** for all artifacts
2. **Separate registries by stability**:
   - `dev` - Unstable, frequent updates
   - `staging` - Release candidates
   - `prod` - Stable, production releases
3. **Configure change log retention** based on team size
4. **Use local registries** for rapid development iteration
5. **Implement registry mirrors** for global teams

## Future Enhancements

- **Registry federation** - Cross-registry discovery
- **Garbage collection** - Automated cleanup of old versions
- **Registry UI** - Web interface for browsing
- **Advanced queries** - Search by capability, API usage
- **Replication** - Multi-region registry sync