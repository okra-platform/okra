# Registry System Design

> **Last Updated**: 2025-07-20
> 
> **Design Review Updates**:
> - Renamed `Registry` to `ArtifactRegistry` to avoid naming conflicts
> - Added `Close()` methods to storage interfaces for resource cleanup
> - Added `Shutdown()` method to `RegistryClient` for graceful shutdown
> - Clarified implementation will be in `internal/registry` package
> - Enhanced error handling patterns and test coverage
> - Added JSON field tags with camelCase naming to comply with OKRA conventions
> - Renamed `Version` to `ArtifactVersion` to avoid confusion with `ServiceVersion`
> - Renamed `CacheManager` to `ArtifactCacheManager` for clarity
> - Added `AWSCredentials` struct definition
> - Clarified JSON-only serialization for WASM boundaries
> - Added comprehensive dependencies section with required Go libraries

## 1. Overview

### Problem Statement
OKRA needs a robust registry system to enable publishing, storing, and distributing service packages, type definitions, and host binaries. While the registry approach is documented, there is no implementation for:
- Publishing packages to registries
- Resolving packages from registries
- Managing registry configurations
- Supporting multiple storage backends (local filesystem, S3)
- Caching and synchronization

### Goals
- Implement a flexible registry client that supports multiple storage backends
- Provide seamless publishing and resolution of OKRA artifacts
- Enable efficient caching and synchronization
- Support both development (local) and production (S3) scenarios
- Maintain backward compatibility with existing package loading

### Non-Goals
- Registry federation (future enhancement)
- Web UI for browsing registries
- Advanced query capabilities beyond basic resolution
- Garbage collection of old versions
- Package signing (future enhancement)

### High-Level Solution
Create a modular registry system with:
1. **Storage abstraction layer** supporting local filesystem and S3 backends
2. **Registry client** for publishing and resolving artifacts
3. **Configuration management** for registry settings
4. **Caching layer** for efficient access
5. **CLI integration** for publish/deploy commands

### Implementation Module
The registry system will be implemented in a dedicated `internal/registry` package to avoid naming conflicts with existing code.

## 2. Interfaces & APIs

### Core Registry Interfaces

```go
// ArtifactRegistry provides access to OKRA artifacts
// Renamed from Registry to avoid conflicts with existing codegen.Registry
type ArtifactRegistry interface {
    // GetName returns the registry name
    GetName() string
    
    // GetTypes returns the artifact types this registry supports
    GetTypes() []ArtifactType
    
    // Publish uploads an artifact to the registry
    Publish(ctx context.Context, artifact Artifact) error
    
    // Resolve finds and downloads an artifact
    Resolve(ctx context.Context, ref ArtifactRef) (Artifact, error)
    
    // List returns available versions for an FQN
    List(ctx context.Context, fqn string) ([]ArtifactVersion, error)
    
    // GetChanges returns changes since a given time
    GetChanges(ctx context.Context, since time.Time) ([]ChangeEntry, error)
}

// RegistryStorage abstracts the underlying storage mechanism
type RegistryStorage interface {
    // Put stores content at the given path
    Put(ctx context.Context, path string, content io.Reader) error
    
    // Get retrieves content from the given path
    Get(ctx context.Context, path string) (io.ReadCloser, error)
    
    // Exists checks if a path exists
    Exists(ctx context.Context, path string) (bool, error)
    
    // List returns entries matching the prefix
    List(ctx context.Context, prefix string) ([]string, error)
    
    // Delete removes content at the given path
    Delete(ctx context.Context, path string) error
    
    // CreateSymlink creates a symbolic link (latest versions)
    CreateSymlink(ctx context.Context, target, link string) error
    
    // Close releases any resources held by the storage
    Close() error
}

// RegistryClient manages multiple registries
type RegistryClient interface {
    // LoadConfig loads registry configuration from disk
    LoadConfig() error
    
    // GetRegistry returns a specific registry by name
    GetRegistry(name string) (ArtifactRegistry, error)
    
    // Resolve searches all registries for an artifact
    Resolve(ctx context.Context, ref ArtifactRef) (Artifact, ArtifactRegistry, error)
    
    // Publish publishes to a specific registry
    Publish(ctx context.Context, registryName string, artifact Artifact) error
    
    // GetCache returns the cache manager
    GetCache() ArtifactCacheManager
    
    // Shutdown gracefully shuts down the registry client
    Shutdown(ctx context.Context) error
}

// ArtifactCacheManager handles local caching of registry artifacts
// Renamed from CacheManager for clarity and to distinguish from other caching mechanisms
type ArtifactCacheManager interface {
    // Get retrieves an artifact from cache
    Get(registryName string, ref ArtifactRef) (Artifact, error)
    
    // Put stores an artifact in cache
    Put(registryName string, ref ArtifactRef, artifact Artifact) error
    
    // Invalidate removes an artifact from cache
    Invalidate(registryName string, ref ArtifactRef) error
    
    // Clear removes all cached artifacts for a registry
    Clear(registryName string) error
    
    // Close releases any resources held by the cache
    Close() error
}
```

### Data Types

```go
// ArtifactType represents the type of registry artifact
type ArtifactType string

const (
    ArtifactTypeService ArtifactType = "service"
    ArtifactTypeType    ArtifactType = "type"
    ArtifactTypeHost    ArtifactType = "host"
)

// Artifact represents a registry artifact
// This is distinct from build.BuildArtifacts which represents build outputs.
// The registry Artifact represents a packaged, versioned artifact ready for distribution.
type Artifact struct {
    Type     ArtifactType           `json:"type"`
    FQN      string                 `json:"fqn"`
    Version  ArtifactVersion        `json:"version"`
    Metadata map[string]interface{} `json:"metadata,omitempty"`
    Content  io.ReadCloser          `json:"-"` // Not serialized
    Checksum string                 `json:"checksum"`
}

// ArtifactRef is a reference to an artifact
type ArtifactRef struct {
    FQN     string            `json:"fqn"`
    Version VersionConstraint `json:"version"` // Can be exact version or constraint like "^1.2.0"
    Type    ArtifactType      `json:"type"`
}

// ArtifactVersion represents a semantic version for artifacts
// Renamed from Version to avoid confusion with ServiceVersion from lifecycle management
type ArtifactVersion struct {
    Major int    `json:"major"`
    Minor int    `json:"minor"`
    Patch int    `json:"patch"`
    Pre   string `json:"pre,omitempty"` // Pre-release identifier
}

// VersionConstraint represents a version requirement
type VersionConstraint interface {
    Matches(v ArtifactVersion) bool
    String() string
}

// ChangeEntry represents a registry change
type ChangeEntry struct {
    FQN             string           `json:"fqn"`
    Version         ArtifactVersion  `json:"version"`
    Type            ArtifactType     `json:"type"`
    UpdatedAt       time.Time        `json:"updatedAt"`
    Summary         string           `json:"summary"`
    PreviousVersion *ArtifactVersion `json:"previousVersion,omitempty"`
}

// RegistryConfig represents registry configuration
type RegistryConfig struct {
    Name     string         `json:"name"`
    Types    []ArtifactType `json:"types"`
    Path     string         `json:"path"`     // URL: file://, s3://, https://
    Region   string         `json:"region"`   // For S3
    CacheTTL time.Duration  `json:"cacheTtl"`
    Auth     AuthConfig     `json:"auth"`
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
    Type        string          `json:"type"`        // "none", "iam", "token"
    Token       string          `json:"token,omitempty"`
    Credentials *AWSCredentials `json:"credentials,omitempty"`
}

// AWSCredentials represents AWS authentication credentials
type AWSCredentials struct {
    AccessKeyID     string `json:"accessKeyId"`
    SecretAccessKey string `json:"secretAccessKey"`
    SessionToken    string `json:"sessionToken,omitempty"`
    Region          string `json:"region"`
}
```

## 3. Component Interactions

### Publishing Flow
```
CLI -> RegistryClient: publish(registry, artifact)
RegistryClient -> ArtifactRegistry: publish(artifact)
ArtifactRegistry -> Storage: put(path, content)
Storage -> S3/FS: upload
ArtifactRegistry -> Storage: createSymlink("latest", version)
ArtifactRegistry -> ChangeLog: recordChange(entry)
```

### Resolution Flow
```
CLI -> RegistryClient: resolve(ref)
RegistryClient -> Cache: get(ref)
Cache -> RegistryClient: (miss)
RegistryClient -> ArtifactRegistry[]: resolve(ref)
ArtifactRegistry -> Storage: list(fqn)
ArtifactRegistry -> Storage: get(path)
Storage -> ArtifactRegistry: content
ArtifactRegistry -> RegistryClient: artifact
RegistryClient -> Cache: put(artifact)
RegistryClient -> CLI: artifact
```

### Configuration Loading
```
RegistryClient -> ConfigLoader: load("~/.okra/registries.yml")
ConfigLoader -> YAML: parse
ConfigLoader -> RegistryClient: []RegistryConfig
RegistryClient -> RegistryFactory: create(config)
RegistryFactory -> LocalRegistry/S3Registry: new
```

## 4. Implementation Approach

### Package Location
The registry system will be implemented in the `internal/registry` package to avoid naming conflicts with existing code (e.g., `codegen.Registry`).

### Version Coordination
The registry's `ArtifactVersion` type is distinct from the `ServiceVersion` type in the service lifecycle management system. The registry `ArtifactVersion` focuses on artifact versioning (semver), while `ServiceVersion` handles runtime service instance versioning.

### Serialization Format
All cross-boundary communication uses JSON serialization, consistent with OKRA's WASM patterns. This applies to:
- Change log entries
- Configuration files
- Cache metadata
- API responses

Note: Protobuf is NOT used at WASM boundaries due to TinyGo's lack of reflection support.

### Storage Implementations

#### Local Storage
```go
type localStorage struct {
    basePath string
}

// Uses os package for file operations
// Symlinks via os.Symlink
// Lists via filepath.Walk
```

#### S3 Storage
```go
type s3Storage struct {
    client     *s3.Client
    bucket     string
    prefix     string
}

// Uses AWS SDK v2
// Symlinks via S3 object metadata
// Lists via S3 ListObjectsV2
```

### Registry Implementation
```go
type registryImpl struct {
    name    string
    types   []ArtifactType
    storage RegistryStorage
    logger  *slog.Logger
}

// FQN to path mapping follows docs/14_registry-approach.md structure
// Version resolution uses semver library
// Change log updates are atomic
```

### Cache Implementation
```go
type artifactCacheManager struct {
    basePath string
    mu       sync.RWMutex
    index    map[string]cacheEntry
}

// Cache structure: ~/.okra/cache/{registry}/{fqn}/{version}/
// Index persisted to disk for fast lookups
// TTL checked on access
```

### Version Resolution Algorithm
1. Parse version constraint (exact, range, latest)
2. List all versions for FQN
3. Sort versions (newest first)
4. Find first matching version
5. Return match or error

### Change Log Management
- Date-based directory structure: `/updates/YYYY/MM/DD/`
- JSON files per FQN update
- Atomic writes with temp file + rename
- Efficient prefix queries for sync

## 5. Test Strategy

### Unit Test Cases
```
// Storage Tests
// Test: Can put and get content
// Test: Can check existence
// Test: Can list with prefix
// Test: Can create symlinks
// Test: Handles missing paths gracefully
// Test: S3 storage handles network errors

// Registry Tests  
// Test: Can publish service package
// Test: Can resolve by exact version
// Test: Can resolve by version constraint
// Test: Can list available versions
// Test: Records changes correctly
// Test: Handles concurrent publishes

// Cache Tests
// Test: Cache hit returns artifact
// Test: Cache miss returns nil
// Test: Respects TTL expiration
// Test: Can invalidate entries
// Test: Handles concurrent access

// Version Tests
// Test: Parses semantic versions correctly
// Test: Version constraints match correctly
// Test: Sorts versions properly
// Test: Handles pre-release versions

// Resource Management Tests
// Test: Storage Close() releases connections
// Test: Cache Close() flushes pending writes
// Test: Client Shutdown() gracefully closes all resources
// Test: No resource leaks under concurrent load
```

### Integration Test Cases
```
// End-to-End Tests
// Test: Can publish and resolve through client
// Test: Resolution order follows configuration
// Test: Cache improves performance
// Test: S3 backend works with real bucket
// Test: Change log enables incremental sync
// Test: Handles registry unavailability

// Concurrent Operation Tests
// Test: Concurrent publishes to same FQN handled correctly
// Test: Concurrent cache access is thread-safe
// Test: Network partition recovery for S3 operations
// Test: Cache corruption detected and recovered
```

### Edge Cases
- Publishing same version twice
- Resolving non-existent package
- Network failures during S3 operations
- Corrupted cache entries
- Invalid version strings
- Missing registry configuration
- Permission errors on local filesystem

## 6. Error Handling

### Error Types
```go
var (
    ErrArtifactNotFound = errors.New("artifact not found")
    ErrVersionConflict  = errors.New("version already exists")
    ErrInvalidVersion   = errors.New("invalid version format")
    ErrRegistryNotFound = errors.New("registry not found")
    ErrStorageFailure   = errors.New("storage operation failed")
    ErrAuthFailure      = errors.New("authentication failed")
)
```

### Error Propagation
- Storage errors wrapped with context using fmt.Errorf("operation failed: %w", err)
- Registry errors include registry name for debugging
- Resolution errors include searched registries list
- Network errors trigger retries with exponential backoff
- All errors follow Go idioms with error wrapping for context

### Recovery Strategies
- Automatic retry for transient S3 errors
- Fallback to other registries on failure
- Cache serves stale entries if registry unavailable
- Clear error messages guide troubleshooting

## 7. Performance Considerations

### Scalability
- Concurrent resolution from multiple registries
- Streaming upload/download for large packages
- Efficient prefix queries for S3 listings
- Connection pooling for S3 client

### Resource Usage
- Memory: Stream large files instead of loading
- CPU: Parallel checksum verification
- Network: Compressed transfers, resumable uploads
- Disk: Configurable cache size limits

### Bottlenecks
- S3 list operations (mitigated by caching)
- Version resolution with many versions
- Change log queries for large time ranges

### Optimization Opportunities
- Bloom filters for negative cache
- Registry mirrors for geo-distribution
- Delta updates for large packages
- Background cache warming

## 8. Security Considerations

### Attack Vectors
- Man-in-the-middle for HTTP registries
- Tampered packages
- Unauthorized publishing
- Cache poisoning
- Path traversal in local storage

### Mitigations
- Checksum verification for all downloads
- TLS required for HTTP registries
- IAM roles for S3 access
- File permissions for local registries
- Input validation for all paths
- Signed URLs for S3 (future)

### Policy Integration
- Registry access controlled by policies
- Service capabilities limit registry usage
- Audit logs for all registry operations

## 9. Dependencies

### Core Libraries

#### AWS SDK v2 (for S3 Storage)
```go
github.com/aws/aws-sdk-go-v2/config
github.com/aws/aws-sdk-go-v2/service/s3
github.com/aws/aws-sdk-go-v2/feature/s3/manager
```
- Used for S3 storage backend implementation
- Provides efficient streaming uploads/downloads
- Supports IAM role authentication

#### Semantic Versioning
```go
github.com/Masterminds/semver/v3
```
- Industry-standard semver parsing and constraint matching
- Handles version sorting and comparison
- Supports pre-release versions and metadata

#### YAML Configuration
```go
gopkg.in/yaml.v3
```
- For parsing registry configuration files
- Already used elsewhere in OKRA

#### File System Operations
```go
github.com/spf13/afero
```
- Abstraction layer for file system operations
- Enables better testing with in-memory filesystems
- Provides atomic file operations

### Existing OKRA Dependencies (to reuse)

#### Logging
```go
log/slog // Standard library structured logging
```
- Already used throughout OKRA
- Provides structured logging with levels

#### Concurrency
```go
sync // Standard library
golang.org/x/sync/errgroup
```
- For concurrent operations and synchronization
- Error group for parallel registry operations

#### Testing
```go
github.com/stretchr/testify
```
- Already used in OKRA for assertions
- Provides require/assert patterns

### Optional Libraries (for future enhancements)

#### Compression
```go
github.com/klauspost/compress
```
- For artifact compression/decompression
- Better performance than standard library

#### Caching
```go
github.com/dgraph-io/ristretto
```
- High-performance cache with TTL support
- Could replace simple map-based cache

#### Checksums
```go
crypto/sha256 // Standard library
encoding/hex  // Standard library
```
- For artifact integrity verification
- SHA-256 checksums for all artifacts

## 10. Open Questions

### Design Decisions
1. Should we support partial package downloads?
2. How to handle registry mirrors?
3. Should change logs be eventually consistent?

### Trade-offs
1. **Caching Strategy**: Aggressive (more disk) vs Conservative (more network)
2. **Version Resolution**: Client-side (flexible) vs Server-side (efficient)
3. **Storage Format**: Flat files vs Database for metadata

### Future Considerations
- Registry federation protocol
- Package signing and verification
- Garbage collection policies
- Advanced search capabilities
- WebAssembly component model support

## 11. Implementation Plan

### Phase 1: Core Registry (Week 1)
1. Storage abstraction and implementations
2. Basic registry operations
3. Version parsing and resolution
4. Unit tests

### Phase 2: Client & Cache (Week 2)
1. Registry client with config loading
2. Cache manager implementation
3. CLI command integration
4. Integration tests

### Phase 3: S3 & Production (Week 3)
1. S3 storage with AWS SDK
2. Change log implementation
3. Authentication support
4. Performance optimization

### Phase 4: Polish & Documentation (Week 4)
1. Error handling improvements
2. Comprehensive documentation
3. Example configurations
4. Migration guide