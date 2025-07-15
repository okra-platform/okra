# Design-First Development Approach

This document outlines the preferred approach for implementing new features in OKRA: creating detailed design documents before writing code.

## Why Design-First?

When implementing new features, we follow a design-first approach that prioritizes:

1. **Faster iteration cycles** - Design documents can be refined quickly without refactoring code
2. **Better architectural decisions** - Thinking through the whole system before implementation
3. **Clearer communication** - Designs are easier to review than code for understanding intent
4. **Parallel development** - Multiple agents/developers can work on different features without conflicts
5. **Reduced rework** - Getting the design right first means less code to throw away

## Design Document Structure

Each feature design document should include the following sections:

### 1. Overview
- **Problem Statement**: What problem are we solving?
- **Goals**: What are the success criteria?
- **Non-Goals**: What are we explicitly NOT trying to solve?
- **High-Level Solution**: Brief description of the approach

### 2. Interfaces & APIs
- **Public Interfaces**: All public-facing interfaces with clear documentation
- **Responsibilities**: What each interface/component is responsible for
- **Boundaries**: Clear delineation of concerns between components

Example:
```go
// WASMWorkerPool manages a pool of WASM workers for concurrent execution
type WASMWorkerPool interface {
    // Invoke executes a method on an available worker
    Invoke(ctx context.Context, method string, input []byte) ([]byte, error)
    
    // Shutdown gracefully shuts down all workers
    Shutdown(ctx context.Context) error
}
```

### 3. Component Interactions
- **Sequence Diagrams**: For complex flows (can use ASCII art or mermaid)
- **Data Flow**: How data moves through the system
- **Lifecycle**: Component creation, initialization, and cleanup

Example:
```
Service -> HostAPI: env.get("API_URL")
HostAPI -> PolicyEngine: evaluate(request)
PolicyEngine -> HostAPI: allowed
HostAPI -> Service: "https://api.example.com"
```

### 4. Implementation Approach
- **Key Algorithms**: Core logic and algorithms
- **Data Structures**: Important data structures and their purpose
- **Design Patterns**: Patterns being used and why
- **Dependencies**: External libraries or internal packages needed

### 5. Test Strategy
- **Unit Test Cases**: Specific scenarios for unit testing
- **Integration Test Cases**: How components work together
- **Edge Cases**: Error conditions and boundary cases
- **Performance Tests**: If applicable

Example test cases:
```
// Test: Can create pool with min/max workers
// Test: Blocks when all workers are busy
// Test: Gracefully handles worker crashes
// Test: Respects context cancellation
```

### 6. Error Handling
- **Error Types**: What kinds of errors can occur
- **Error Propagation**: How errors flow through the system
- **Recovery Strategies**: How to handle and recover from errors

### 7. Performance Considerations
- **Scalability**: How does this scale with load?
- **Resource Usage**: Memory, CPU, network considerations
- **Bottlenecks**: Potential performance bottlenecks
- **Optimization Opportunities**: Areas for future optimization

### 8. Security Considerations
- **Attack Vectors**: Potential security risks
- **Mitigations**: How we protect against these risks
- **Policy Integration**: How security policies are enforced

### 9. Open Questions
- **Design Decisions**: Areas needing input or clarification
- **Trade-offs**: Options with pros/cons that need discussion
- **Future Considerations**: Things to think about but not implement now

## Process

1. **Design Review**: Create design document and get feedback
2. **Iterate**: Refine based on feedback until approved
3. **Implementation**: Proceed with coding based on approved design
4. **Code Review**: Ensure implementation matches design

## Benefits for Multi-Agent Development

With clear design documents:
- Multiple agents can work on different features simultaneously
- Dependencies between features are identified early
- Integration points are well-defined
- Reduces merge conflicts and integration issues

## Design Document Location

Design documents should be placed in:
- `docs/design/` for feature designs
- Include date and feature name: `YYYY-MM-DD-feature-name.md`
- Link from main documentation once implemented

## Multi-Agent Git Workflow

When working with multiple Claude agents on different features, each agent should work on its own branch to avoid conflicts.

### Branch Naming Convention
- Feature branches: `feature/<feature-name>`
- Bug fixes: `fix/<issue-description>`
- Creating design docs: `design-doc/<feature-name>`

### Claude Command Format

To start a new agent on a feature, use this command format in a new Claude tab:

```
Please implement the feature "<feature-name>" by:
1. Reading the design document at docs/design/YYYY-MM-DD-<feature-name>.md
2. Creating a new git branch called feature/<feature-name>
3. Implementing the feature according to the design document
4. Following all OKRA conventions in docs/100_coding-conventions.md and docs/102_testing-best-practices.md
5. Running tests after implementation
Do NOT commit changes - I will handle commits after review.
```


### What Claude Will Do

1. **Check current branch** - Verify starting from the right place
2. **Create feature branch** - `git checkout -b feature/<feature-name>`
3. **Read design document** - Load and understand the approved design
4. **Read coding conventions** - Ensure compliance with project standards
5. **Implement the feature** - Following the design exactly
6. **Write comprehensive tests** - As specified in the design
7. **Run tests** - Ensure everything passes
8. **Report completion** - Summary of what was implemented

### Pre-Flight Checklist for Starting Multiple Agents

Before starting multiple agents:
1. Ensure all design documents are complete and approved
2. Make sure main branch is up to date
3. Check that feature names are unique and descriptive
4. Verify no existing branches with the same names

### Managing Multiple Agents

Tips for managing multiple Claude agents:
- **One feature per tab** - Keep each Claude session focused on one feature
- **Clear feature names** - Use descriptive names that won't conflict
- **Check dependencies** - If features depend on each other, implement in order
- **Regular syncs** - Periodically check that branches don't diverge too much

### After Implementation

Once Claude completes the implementation:
1. Review the changes locally
2. Run additional tests if needed
3. Create commits with appropriate messages
4. Create pull requests for review
5. Handle any merge conflicts before merging

### Troubleshooting

If an agent gets confused about which branch it's on:
```
# Claude can run:
git status
git branch --show-current
```

If you need to restart an agent's work:
```
# Have Claude run:
git checkout main
git branch -D feature/<feature-name>
# Then give the implementation command again
```

