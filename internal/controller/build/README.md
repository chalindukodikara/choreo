# Build Controller Refactored Architecture

This document describes the refactored build controller architecture that separates concerns and provides extensibility for different build engines.

## Architecture Overview

The refactored build controller follows a layered architecture with clear separation of concerns:

```
┌─────────────────────────────────────────┐
│           Controller Layer              │
│  (controller.go, controller_conditions) │
└─────────────────┬───────────────────────┘
                  │
┌─────────────────▼───────────────────────┐
│            Service Layer                │
│     (service/build_service.go)          │
└─────────────────┬───────────────────────┘
                  │
┌─────────────────▼───────────────────────┐
│          Engine Interface               │
│      (interfaces/builder.go)            │
└─────────────────┬───────────────────────┘
                  │
┌─────────────────▼───────────────────────┐
│       Concrete Implementations          │
│   engines/argo/    engines/tekton/      │
└─────────────────────────────────────────┘
```

## Directory Structure

```
internal/controller/build/
├── controller.go              # Main controller (orchestrates reconciliation)
├── controller_conditions.go   # Legacy condition helpers (to be migrated)
├── interfaces/
│   └── builder.go             # BuildEngine interface definition
├── service/
│   ├── build_service.go       # Business logic and engine orchestration
│   └── conditions.go          # Condition management
├── engines/
│   ├── argo/
│   │   ├── engine.go          # Argo Workflows implementation
│   │   └── workflow.go        # Argo-specific workflow creation
│   └── tekton/
│       └── engine.go          # Tekton Pipelines implementation (stub)
└── utils/
    └── naming.go              # Shared utilities for naming and normalization
```

## Key Components

### 1. Controller Layer (`controller.go`)
- **Responsibility**: Kubernetes reconciliation loop, status updates, condition management
- **Key Features**:
  - Handles Kubernetes-specific concerns (resource fetching, status updates)
  - Delegates business logic to service layer
  - Manages build lifecycle state transitions
  - Maintains backward compatibility with existing flow

### 2. Service Layer (`service/build_service.go`)
- **Responsibility**: Business logic, engine selection, artifact management
- **Key Features**:
  - Engine registry and selection logic
  - Build process orchestration
  - Artifact extraction and workload creation
  - Build status management
  - Abstract interface to different build engines

### 3. Engine Interface (`interfaces/builder.go`)
- **Responsibility**: Defines contract for build engine implementations
- **Key Features**:
  - Standard interface for all build engines
  - Generic build phases and status representation
  - Artifact extraction abstraction
  - Engine-agnostic build information

### 4. Engine Implementations
#### Argo Engine (`engines/argo/`)
- **Responsibility**: Argo Workflows-specific implementation
- **Key Features**:
  - Creates Argo Workflow resources
  - Manages Argo-specific prerequisites (RBAC, namespaces)
  - Extracts artifacts from workflow outputs
  - Converts Argo phases to generic build phases

#### Tekton Engine (`engines/tekton/`) - Future
- **Responsibility**: Tekton Pipelines implementation (stub for now)
- **Features**: Will implement PipelineRun creation and management

### 5. Utilities (`utils/`)
- **Responsibility**: Shared helper functions
- **Key Features**:
  - Naming conventions and normalization
  - Kubernetes resource generation helpers
  - Image name/tag generation

## Benefits of Refactored Architecture

### 1. **Separation of Concerns**
- **Controller**: Focuses solely on Kubernetes reconciliation
- **Service**: Handles business logic and engine orchestration  
- **Engines**: Implement build-engine-specific logic
- **Utils**: Provide shared utilities

### 2. **Extensibility**
- Easy to add new build engines (Tekton, Jenkins X, Custom)
- Engine selection can be based on build spec, annotations, or configuration
- Each engine can have its own specific requirements and implementations

### 3. **Testability**
- Service layer can be unit tested independently
- Engine implementations can be tested in isolation
- Mock engines can be created for testing
- Clear interfaces make dependency injection easier

### 4. **Maintainability**
- Engine-specific logic is contained within respective packages
- Shared utilities avoid code duplication
- Clear boundaries between layers
- Easier to debug and troubleshoot specific engines

### 5. **Backward Compatibility**
- Existing build flow and logic preserved
- Same API and behavior from controller perspective
- Gradual migration path for any future changes

## Engine Selection Logic

The service layer determines which build engine to use based on:

1. **Build Spec**: `build.Spec.TemplateRef.Engine` field
2. **Default**: Falls back to "argo" if not specified
3. **Future**: Can be extended to support:
   - Organization/project defaults
   - Build plane configuration
   - Annotation-based selection

## Adding New Build Engines

To add a new build engine (e.g., Jenkins X):

1. **Create engine package**: `engines/jenkinsx/engine.go`
2. **Implement BuildEngine interface**:
   ```go
   type Engine struct { ... }
   func (e *Engine) GetName() string { return "jenkinsx" }
   func (e *Engine) EnsurePrerequisites(...) error { ... }
   func (e *Engine) CreateBuild(...) (BuildInfo, error) { ... }
   func (e *Engine) GetBuildStatus(...) (BuildStatus, error) { ... }
   func (e *Engine) ExtractBuildArtifacts(...) (*BuildArtifacts, error) { ... }
   ```
3. **Register in service**: Add to `registerBuildEngines()` method
4. **Update engine selection**: Modify `determineBuildEngine()` if needed

## Migration Notes

### Phase 1: ✅ Complete
- Created new architecture with interfaces and service layer
- Moved Argo-specific logic to dedicated engine
- Refactored controller to use service layer
- Maintained backward compatibility

### Phase 2: Future
- Implement Tekton engine
- Add engine selection configuration
- Migrate remaining utility functions
- Add comprehensive tests

### Phase 3: Future  
- Add support for custom build engines
- Implement plugin architecture
- Add engine-specific configuration options
- Performance optimizations

## Testing Strategy

```go
// Engine testing
func TestArgoEngine_CreateBuild(t *testing.T) { /* ... */ }

// Service testing with mock engines
func TestBuildService_ProcessBuild(t *testing.T) {
    mockEngine := &MockBuildEngine{}
    service := NewBuildServiceWithEngines(map[string]BuildEngine{
        "mock": mockEngine,
    })
    // ...
}

// Controller integration testing
func TestReconciler_Reconcile(t *testing.T) { /* ... */ }
```

This refactored architecture provides a solid foundation for extending the build controller to support multiple build engines while maintaining clean separation of concerns and backward compatibility.
