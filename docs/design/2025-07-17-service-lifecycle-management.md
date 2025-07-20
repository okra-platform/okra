# Service Lifecycle Management Design

## Overview

This document outlines the design for comprehensive service lifecycle management in OKRA, providing production-grade capabilities for health checking, graceful shutdown, versioning, and zero-downtime deployments. These features are essential for operating OKRA services in production environments where reliability and availability are critical.

## Goals

1. **Health Monitoring** - Enable deep health checks for services with customizable probes
2. **Graceful Operations** - Ensure clean startup/shutdown with proper resource cleanup
3. **Zero Downtime** - Support rolling updates without service interruption
4. **Version Management** - Track and manage multiple service versions simultaneously
5. **Operational Visibility** - Provide clear insights into service state and transitions

## Non-Goals

1. **Service Mesh Integration** - Not implementing Istio/Linkerd specific features
2. **Container Orchestration** - Not replacing Kubernetes controllers
3. **Configuration Management** - Not implementing GitOps or config sync
4. **Auto-scaling** - Covered by actor system's existing capabilities
5. **Multi-region Failover** - Beyond current scope

## Design

### 1. Health Check System

#### Health Check Types

```go
// HealthCheckType defines the type of health check
type HealthCheckType string

const (
    // Liveness - is the service running and not deadlocked?
    HealthCheckLiveness HealthCheckType = "liveness"
    
    // Readiness - is the service ready to accept traffic?
    HealthCheckReadiness HealthCheckType = "readiness"
    
    // Startup - is the service still starting up?
    HealthCheckStartup HealthCheckType = "startup"
)

// HealthStatus represents the health state
type HealthStatus string

const (
    HealthStatusHealthy   HealthStatus = "healthy"
    HealthStatusUnhealthy HealthStatus = "unhealthy"
    HealthStatusDegraded  HealthStatus = "degraded"
)
```

#### Service-Defined Health Checks

Services can implement custom health checks via a special GraphQL query:

```graphql
type Query {
  # Standard health check endpoint
  _health(type: HealthCheckType!): HealthCheckResult! @internal
}

type HealthCheckResult {
  status: HealthStatus!
  message: String
  details: JSON
  dependencies: [DependencyHealth!]
}

type DependencyHealth {
  name: String!
  status: HealthStatus!
  message: String
}

enum HealthCheckType {
  LIVENESS
  READINESS
  STARTUP
}

enum HealthStatus {
  HEALTHY
  UNHEALTHY
  DEGRADED
}
```

#### Implementation Example

```go
// In generated service code
func (s *Service) _Health(ctx context.Context, args struct{ Type HealthCheckType }) (*HealthCheckResult, error) {
    switch args.Type {
    case HealthCheckLiveness:
        // Basic liveness - are we responsive?
        return &HealthCheckResult{
            Status: HealthStatusHealthy,
            Message: "Service is alive",
        }, nil
        
    case HealthCheckReadiness:
        // Check dependencies
        deps := []DependencyHealth{}
        
        // Check database connection
        if err := s.checkDatabase(); err != nil {
            deps = append(deps, DependencyHealth{
                Name: "database",
                Status: HealthStatusUnhealthy,
                Message: err.Error(),
            })
        }
        
        // Check external APIs
        if err := s.checkExternalAPIs(); err != nil {
            deps = append(deps, DependencyHealth{
                Name: "external-api",
                Status: HealthStatusDegraded,
                Message: err.Error(),
            })
        }
        
        // Determine overall status
        status := HealthStatusHealthy
        if hasUnhealthy(deps) {
            status = HealthStatusUnhealthy
        } else if hasDegraded(deps) {
            status = HealthStatusDegraded
        }
        
        return &HealthCheckResult{
            Status: status,
            Message: "Readiness check complete",
            Dependencies: deps,
        }, nil
        
    case HealthCheckStartup:
        // Check if initialization is complete
        if !s.initialized {
            return &HealthCheckResult{
                Status: HealthStatusUnhealthy,
                Message: "Service still initializing",
                Details: map[string]interface{}{
                    "progress": s.initProgress,
                },
            }, nil
        }
        return &HealthCheckResult{
            Status: HealthStatusHealthy,
            Message: "Startup complete",
        }, nil
    }
}
```

#### Runtime Health Check Integration

```go
// In runtime package
type HealthChecker interface {
    CheckHealth(ctx context.Context, checkType HealthCheckType) (*HealthCheckResult, error)
}

// ServiceHealth manages health checks for a service
type ServiceHealth struct {
    service      ServiceInstance
    config       HealthCheckConfig
    lastResults  map[HealthCheckType]*HealthCheckResult
    mu           sync.RWMutex
}

type HealthCheckConfig struct {
    // How often to run checks
    Interval time.Duration
    
    // Timeout for each check
    Timeout time.Duration
    
    // Number of consecutive failures before marking unhealthy
    FailureThreshold int
    
    // Number of consecutive successes before marking healthy
    SuccessThreshold int
    
    // Initial delay before first check
    InitialDelay time.Duration
}
```

### 2. Graceful Shutdown

#### Shutdown Sequence

```go
// ShutdownManager coordinates graceful shutdown
type ShutdownManager interface {
    // InitiateShutdown starts the shutdown process
    InitiateShutdown(ctx context.Context, reason string) error
    
    // RegisterShutdownHook adds a cleanup function
    RegisterShutdownHook(name string, hook ShutdownHook)
    
    // WaitForShutdown blocks until shutdown is complete
    WaitForShutdown() error
}

type ShutdownHook func(ctx context.Context) error

// Shutdown phases
type ShutdownPhase string

const (
    // Stop accepting new requests
    ShutdownPhaseStopAccepting ShutdownPhase = "stop_accepting"
    
    // Wait for in-flight requests to complete
    ShutdownPhaseDrainConnections ShutdownPhase = "drain_connections"
    
    // Stop background tasks
    ShutdownPhaseStopBackground ShutdownPhase = "stop_background"
    
    // Cleanup resources
    ShutdownPhaseCleanup ShutdownPhase = "cleanup"
    
    // Final shutdown
    ShutdownPhaseFinal ShutdownPhase = "final"
)
```

#### Service Shutdown Hooks

Services can define cleanup logic:

```graphql
type Mutation {
  # Called during graceful shutdown
  _shutdown(phase: ShutdownPhase!): ShutdownResult @internal
}

type ShutdownResult {
  ready: Boolean!
  message: String
  pendingOperations: Int
}

enum ShutdownPhase {
  STOP_ACCEPTING
  DRAIN_CONNECTIONS
  STOP_BACKGROUND
  CLEANUP
  FINAL
}
```

#### Connection Draining

```go
// ConnectionDrainer manages in-flight requests during shutdown
type ConnectionDrainer struct {
    activeRequests sync.WaitGroup
    draining       atomic.Bool
    maxDrainTime   time.Duration
}

func (cd *ConnectionDrainer) StartDraining() {
    cd.draining.Store(true)
}

func (cd *ConnectionDrainer) WaitForDrain(ctx context.Context) error {
    done := make(chan struct{})
    go func() {
        cd.activeRequests.Wait()
        close(done)
    }()
    
    select {
    case <-done:
        return nil
    case <-ctx.Done():
        return fmt.Errorf("drain timeout after %v", cd.maxDrainTime)
    }
}

// Middleware for tracking requests
func (cd *ConnectionDrainer) TrackRequest(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if cd.draining.Load() {
            http.Error(w, "Service is shutting down", http.StatusServiceUnavailable)
            return
        }
        
        cd.activeRequests.Add(1)
        defer cd.activeRequests.Done()
        
        next.ServeHTTP(w, r)
    })
}
```

### 3. Service Versioning

#### Version Metadata

```go
// ServiceVersion represents a specific version of a service
type ServiceVersion struct {
    ServiceName string    `json:"service_name"`
    Version     string    `json:"version"`      // Semantic version
    GitCommit   string    `json:"git_commit"`   // Git SHA
    BuildTime   time.Time `json:"build_time"`
    Deprecated  bool      `json:"deprecated"`
    MinOKRAVersion string `json:"min_okra_version"`
}

// VersionManager manages multiple versions of a service
type VersionManager interface {
    // RegisterVersion registers a new service version
    RegisterVersion(ctx context.Context, version ServiceVersion) error
    
    // GetActiveVersions returns all active versions
    GetActiveVersions(serviceName string) ([]ServiceVersion, error)
    
    // RouteToVersion routes requests to specific version
    RouteToVersion(ctx context.Context, serviceName, version string) (ServiceInstance, error)
    
    // DeprecateVersion marks a version as deprecated
    DeprecateVersion(ctx context.Context, serviceName, version string) error
}
```

#### Version Routing

```graphql
# Services can specify version constraints
type Query {
  # Get compatible service version
  _getServiceVersion(
    constraint: String!  # e.g., ">=1.2.0 <2.0.0"
  ): ServiceVersion! @internal
}
```

### 4. Deployment Strategies

#### Blue-Green Deployment

```go
// BlueGreenDeployment manages blue-green deployments
type BlueGreenDeployment struct {
    blue   *ServiceGroup
    green  *ServiceGroup
    active *ServiceGroup // Points to blue or green
    mu     sync.RWMutex
}

type ServiceGroup struct {
    Version   ServiceVersion
    Instances []ServiceInstance
    Health    HealthStatus
}

func (bgd *BlueGreenDeployment) Deploy(ctx context.Context, newVersion ServiceVersion) error {
    // 1. Deploy to inactive group
    inactive := bgd.getInactiveGroup()
    if err := inactive.Deploy(ctx, newVersion); err != nil {
        return fmt.Errorf("deployment failed: %w", err)
    }
    
    // 2. Health check new version
    if err := bgd.waitForHealthy(ctx, inactive); err != nil {
        return fmt.Errorf("health check failed: %w", err)
    }
    
    // 3. Switch traffic
    if err := bgd.switchTraffic(inactive); err != nil {
        return fmt.Errorf("traffic switch failed: %w", err)
    }
    
    // 4. Shutdown old version
    if err := bgd.shutdownGroup(ctx, bgd.getInactiveGroup()); err != nil {
        // Log but don't fail - deployment succeeded
        log.Printf("warning: old version shutdown failed: %v", err)
    }
    
    return nil
}
```

#### Rolling Update

```go
// RollingUpdate manages rolling updates
type RollingUpdate struct {
    maxUnavailable int // Maximum instances that can be down
    maxSurge       int // Maximum extra instances during update
}

func (ru *RollingUpdate) Execute(ctx context.Context, instances []ServiceInstance, newVersion ServiceVersion) error {
    totalInstances := len(instances)
    batchSize := calculateBatchSize(totalInstances, ru.maxUnavailable)
    
    for i := 0; i < totalInstances; i += batchSize {
        end := min(i+batchSize, totalInstances)
        batch := instances[i:end]
        
        // 1. Create new instances (respecting maxSurge)
        newInstances := make([]ServiceInstance, len(batch))
        for j, old := range batch {
            new, err := ru.createNewInstance(ctx, newVersion)
            if err != nil {
                return ru.rollback(ctx, instances[:i], newInstances[:j])
            }
            newInstances[j] = new
        }
        
        // 2. Wait for new instances to be healthy
        if err := ru.waitForHealthy(ctx, newInstances); err != nil {
            return ru.rollback(ctx, instances[:i], newInstances)
        }
        
        // 3. Shift traffic to new instances
        for j, new := range newInstances {
            if err := ru.shiftTraffic(ctx, batch[j], new); err != nil {
                return ru.rollback(ctx, instances[:i], newInstances)
            }
        }
        
        // 4. Shutdown old instances
        for _, old := range batch {
            if err := old.Shutdown(ctx); err != nil {
                log.Printf("warning: failed to shutdown old instance: %v", err)
            }
        }
    }
    
    return nil
}
```

### 5. State Management During Lifecycle

#### State Transfer

```go
// StateTransferable allows services to export/import state
type StateTransferable interface {
    // ExportState exports the current state
    ExportState(ctx context.Context) ([]byte, error)
    
    // ImportState imports state from another instance
    ImportState(ctx context.Context, state []byte) error
}

// In service implementation
func (s *Service) ExportState(ctx context.Context) ([]byte, error) {
    state := ServiceState{
        Version:   s.version,
        Data:      s.inMemoryData,
        Timestamp: time.Now(),
    }
    return json.Marshal(state)
}

func (s *Service) ImportState(ctx context.Context, stateData []byte) error {
    var state ServiceState
    if err := json.Unmarshal(stateData, &state); err != nil {
        return fmt.Errorf("invalid state data: %w", err)
    }
    
    // Validate version compatibility
    if !s.isCompatibleVersion(state.Version) {
        return fmt.Errorf("incompatible state version: %s", state.Version)
    }
    
    s.inMemoryData = state.Data
    return nil
}
```

### 6. CLI Integration

New CLI commands for lifecycle management:

```bash
# Health check commands
okra health <service-name> [--type=liveness|readiness|startup]
okra health status  # Show all service health

# Deployment commands
okra deploy <service-name> <version> [--strategy=rolling|blue-green]
okra deploy status <deployment-id>
okra deploy rollback <deployment-id>

# Version management
okra version list <service-name>
okra version deprecate <service-name> <version>
okra version route <service-name> <version> --weight=50

# Lifecycle operations
okra service restart <service-name> [--graceful]
okra service drain <service-name>  # Drain connections
okra service scale <service-name> --replicas=3
```

### 7. Observability Integration

#### Metrics

```go
// Lifecycle metrics
var (
    serviceHealth = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "okra_service_health",
            Help: "Service health status (1=healthy, 0=unhealthy)",
        },
        []string{"service", "version", "check_type"},
    )
    
    deploymentDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "okra_deployment_duration_seconds",
            Help: "Time taken to complete deployment",
        },
        []string{"service", "strategy", "status"},
    )
    
    shutdownDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "okra_shutdown_duration_seconds",
            Help: "Time taken for graceful shutdown",
        },
        []string{"service", "phase"},
    )
)
```

#### Events

```go
// Lifecycle events
type LifecycleEvent struct {
    Type        string    `json:"type"`        // health_change, deployment_start, etc.
    Service     string    `json:"service"`
    Version     string    `json:"version"`
    Timestamp   time.Time `json:"timestamp"`
    Details     map[string]interface{} `json:"details"`
}

// Event types
const (
    EventHealthChange      = "health_change"
    EventDeploymentStart   = "deployment_start"
    EventDeploymentComplete = "deployment_complete"
    EventShutdownInitiated = "shutdown_initiated"
    EventShutdownComplete  = "shutdown_complete"
    EventVersionDeprecated = "version_deprecated"
)
```

## Implementation Plan

### Week 1: Health Check System
- [ ] Define health check interfaces and types
- [ ] Implement service-side health check support
- [ ] Create runtime health checker with periodic checks
- [ ] Add health check endpoints to admin API
- [ ] Implement CLI health commands

### Week 2: Graceful Shutdown
- [ ] Design shutdown phase management
- [ ] Implement connection draining
- [ ] Add service shutdown hooks
- [ ] Create shutdown manager
- [ ] Test shutdown scenarios

### Week 3: Deployment Strategies
- [ ] Implement version manager
- [ ] Create blue-green deployment logic
- [ ] Build rolling update mechanism
- [ ] Add deployment CLI commands
- [ ] Handle rollback scenarios

### Week 4: Polish and Testing
- [ ] State transfer between versions
- [ ] Comprehensive integration tests
- [ ] Performance testing of deployments
- [ ] Documentation and examples
- [ ] Production readiness review

## Testing Strategy

### Unit Tests
- Health check logic with various states
- Shutdown sequencing
- Version compatibility checks
- Deployment strategy calculations

### Integration Tests
- Full lifecycle scenarios
- Multi-version deployments
- Failure during deployment
- Network partition handling

### Load Tests
- Connection draining under load
- Rolling updates with traffic
- Health check performance impact

## Security Considerations

1. **Health Endpoint Protection** - Internal-only access to detailed health data
2. **Version Verification** - Cryptographic signatures on deployments
3. **State Transfer Security** - Encrypted state during transfers
4. **Rollback Authorization** - RBAC for deployment operations

## Future Enhancements

1. **Canary Deployments** - Gradual traffic shifting with metrics
2. **A/B Testing** - Route by user segments
3. **Circuit Breakers** - Automatic failure isolation
4. **Deployment Webhooks** - Integration with CI/CD
5. **State Synchronization** - Real-time state sync between versions

## Decision Log

1. **Why GraphQL for health checks?** - Consistency with service definition pattern
2. **Why not use Kubernetes probes directly?** - OKRA runs in multiple environments
3. **Why support multiple deployment strategies?** - Different services have different requirements
4. **Why manual state transfer?** - Explicit control over stateful migrations

## Conclusion

This design provides comprehensive lifecycle management for OKRA services, enabling production-grade deployments with zero downtime, health monitoring, and graceful operations. The implementation follows OKRA's patterns of actor-based systems while providing familiar operational capabilities.