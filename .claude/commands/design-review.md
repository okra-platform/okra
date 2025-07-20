# Design Review Command

Perform a comprehensive review of the specified design document to ensure consistency, correctness, and implementation readiness.

## Usage
```
design-review <feature-name>
```

## Prerequisites
Before reviewing, familiarize yourself with:
- **CLAUDE.md** - Project-specific conventions and requirements
- **docs/100_coding-conventions.md** - Coding standards
- **docs/101_testing-strategy.md** - Testing approach (if applicable)

## Review Checklist

### 1. Document Overview
The document will can be found here: docs/design/*-<feature-name>.md

First, read the entire document to understand:
- Overall design and architecture
- Main components and their interactions
- External dependencies and integration points

### 2. Core Design Principles
Evaluate the design against these principles:
- **Single Responsibility** - Each component should have one clear purpose
- **Clear Interfaces** - All APIs should be well-defined with obvious contracts
- **Proper Abstraction** - Repeated patterns should be abstracted appropriately
- **Loose Coupling** - Components should interact through well-defined boundaries
- **Consistency** - Similar problems should be solved in similar ways

### 3. Technical Validation

**IMPORTANT**: Think deeply about and pay special attention to parts of the design that might be contradictory or create mismatched or unclear requirements during implementation and call those out as well.

#### API Design
- Function signatures make sense (parameters, return types)
- Error handling is consistent and follows Go conventions
- Resource lifecycle is clear (creation, cleanup with Close/Shutdown)
- Context propagation is handled correctly

#### Type System
- No duplicate type definitions
- Consistent naming across the codebase
- Proper use of interfaces vs concrete types
- JSON tags follow conventions (see docs/100_coding-conventions.md)

#### Cross-Boundary Communication (WASM/Network)
- Appropriate serialization format (e.g., JSON not protobuf for WASM)
- Memory safety across boundaries
- Error propagation mechanisms
- Proper memory allocation/deallocation patterns

### 4. Implementation Details

#### Code Examples
- Syntax is correct and would compile
- Import paths are accurate
- Method calls match interface definitions
- Error handling included where appropriate

#### Convention Compliance
Verify the design follows established conventions:
- Review **CLAUDE.md** for project-specific requirements
- Check **docs/100_coding-conventions.md** for:
  - Public interface/private implementation pattern
  - JSON camelCase naming convention
- Reference **docs/101_testing-strategy.md** if design includes test patterns
- For WASM-specific patterns, check existing implementations in `internal/wasm/`

### 5. Common Issues Checklist

Search specifically for these patterns:
- [ ] Duplicate type definitions (search each major type name)
- [ ] Inconsistent lifecycle patterns (Init/Initialize, Close/Shutdown)
- [ ] Missing defensive programming (nil checks, closed state)
- [ ] Unhandled JSON marshaling/unmarshaling errors
- [ ] Factory pattern consistency (if used)
- [ ] Proper error type definitions and usage
- [ ] Memory leaks in cross-boundary calls
- [ ] Race conditions in concurrent code

## Output Format

```markdown
## Design Review Summary

**Document**: docs/design/YYYY-MM-DD-feature-name.md
**Status**: ✅ Ready | ⚠️ Minor fixes needed | ❌ Major revision required

### Critical Issues
[Only include if status is not ✅]
1. **[Category]**: Brief description
   - Location: Section/line reference
   - Impact: Why this matters
   - Fix: Specific action needed

### Recommendations
- [Improvements that would enhance the design]
- [Non-blocking suggestions]

### Strengths
- [What the design does well]
```

## Reference Examples

Common issues from past reviews:
- "Protobuf in WASM boundary (TinyGo doesn't support reflection)"
- "Registry pattern stores instances instead of factories"
- "JSON tags use snake_case instead of camelCase"
- "Missing Close() on resource-holding types"
- "Context values accessed without type assertions"

The goal is to catch issues before implementation begins, ensuring a smooth development process.