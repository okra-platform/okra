# Development and Debugging Guide

This guide covers development tools, environment variables, and debugging techniques for OKRA development.

## Environment Variables

OKRA supports several environment variables to aid in development and debugging:

### `OKRA_KEEP_BUILD_DIR`

When building WASM modules, OKRA creates temporary build directories that are normally cleaned up after the build completes. Setting this environment variable preserves these directories for debugging purposes.

**Usage:**
```bash
OKRA_KEEP_BUILD_DIR=1 okra dev
```

**Purpose:**
- Inspect generated wrapper code
- Debug build failures
- Examine intermediate build artifacts
- Troubleshoot module resolution issues

**Example:**
```bash
# Enable build directory preservation
export OKRA_KEEP_BUILD_DIR=1

# Run okra dev
okra dev

# The build directory path will be logged:
# {"level":"info","component":"go-builder","path":"/var/folders/.../okra-go-build-123456","time":"...","message":"OKRA_KEEP_BUILD_DIR is set - preserving temp build directory for debugging"}
```

The preserved directory contains:
- `main.go` - The generated WASI wrapper
- `go.mod` - The temporary module file
- Copied source files and dependencies
- Any build artifacts

### Future Environment Variables

Additional debugging environment variables may be added in future releases:
- `OKRA_LOG_LEVEL` - Control logging verbosity
- `OKRA_DEBUG` - Enable debug mode with additional diagnostics
- `OKRA_TRACE` - Enable request tracing

## Development Server (`okra dev`)

The development server provides a hot-reload development experience with comprehensive error reporting.

### Error Reporting

The dev server now includes enhanced error reporting:

1. **Tool Verification** - Checks for required tools before starting:
   - TinyGo (for Go projects)
   - Node.js (for TypeScript projects)
   - buf CLI (for all projects)

2. **Configuration Validation** - Validates your `okra.json` configuration:
   - Schema file existence
   - Source path existence
   - Supported language

3. **Build Error Details** - Provides detailed error messages with troubleshooting tips

### Debugging Failed Builds

When a build fails, the dev server provides:

1. **Clear Error Messages** - Specific error details about what failed
2. **Troubleshooting Tips** - Suggestions for common issues
3. **Debug Logging** - Use the logger output for detailed diagnostics

Example error output:
```
❌ Initial build failed: TinyGo build failed: exit status 1

💡 Troubleshooting tips:
   - Check that your schema file exists and is valid
   - Ensure your source files match the configured language
   - Verify all required tools are installed (tinygo, buf)
```

### Debugging Integration Tests

When running integration tests, you can preserve the test directories:

```bash
# Keep test build directories for inspection
OKRA_KEEP_BUILD_DIR=1 go test -v -tags=integration ./test/integration/dev
```

## Common Debugging Scenarios

### 1. Module Resolution Issues

If you encounter Go module import errors:
- Set `OKRA_KEEP_BUILD_DIR=1` and inspect the generated `main.go`
- Check the import paths match your module structure
- Verify your `go.mod` is in the project root

### 2. WASM Compilation Failures

For TinyGo compilation issues:
- Ensure you're using TinyGo-compatible packages
- Check for unsupported Go features (see TinyGo documentation)
- Inspect the build directory for the exact compilation command

### 3. Service Registration Failures

If services aren't accessible:
- Check the actor ID format matches: `namespace.ServiceName.version`
- Verify the schema service name matches the protobuf registration
- Ensure method names match exactly (case-sensitive)

### 4. Hot Reload Not Working

If file changes aren't detected:
- Check the `dev.watch` patterns in `okra.json`
- Ensure files aren't in `dev.exclude` patterns
- Verify file system events are working (some network drives may not support watching)

## Testing and Debugging Tools

### Running Specific Tests

```bash
# Run a specific test
go test -v ./internal/runtime -run TestOkraRuntime_GetActorPID

# Run integration tests with verbose output
go test -v -tags=integration ./test/integration/dev

# Run tests with race detection
go test -race ./...
```

### Debugging Test Failures

1. **Preserve Test Artifacts** - Test output is automatically cleaned up. To preserve:
   ```bash
   # Test logs won't be deleted if the test fails
   go test -v ./... 2>&1 | tee test_output.log
   ```

2. **Enable Debug Logging** - Many components support debug logging via zerolog

3. **Use Test Breakpoints** - Standard Go debugging tools work with OKRA tests

## Performance Debugging

### Actor System Metrics

The GoAKT actor system provides metrics that can be used for debugging:
- Actor mailbox sizes
- Message processing times
- Actor lifecycle events

### WASM Performance

Monitor WASM execution performance:
- Use the duration field in `ServiceResponse` messages
- Check for timeout configurations
- Monitor memory usage in WASM modules

## Reporting Issues

When reporting issues, please include:

1. **Environment Details**
   - Go version (`go version`)
   - TinyGo version (`tinygo version`)
   - OS and architecture

2. **Configuration**
   - Your `okra.json` file
   - Relevant schema files

3. **Debug Output**
   - Set `OKRA_KEEP_BUILD_DIR=1` and include build directory contents
   - Include full error messages and stack traces
   - Provide minimal reproduction steps

## Contributing

When contributing to OKRA:

1. **Follow Testing Guidelines** - See `docs/101_testing-strategy.md`
2. **Use Coding Conventions** - See `docs/100_coding-conventions.md`
3. **Add Debug Helpers** - Consider what debugging would help future developers
4. **Document Environment Variables** - Update this guide for new debugging features