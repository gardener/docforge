# Core Dependency Whitelist

This directory contains tools to enforce architectural boundaries for the core package dependencies.

## Purpose

The core package (`pkg/core/`) contains the main business logic and should have controlled dependencies to:
- Prevent architectural drift
- Avoid unexpected side effects from new dependencies
- Maintain clear separation of concerns
- Enable safe AI agent modifications

## Files

- `check-core-deps.sh` - Script that validates core package dependencies
- `core-deps-whitelist.txt` - List of approved dependencies for core packages

## Usage

### Check Dependencies
```bash
# Run dependency check
make check-core-deps

# Or run directly
./scripts/check-core-deps.sh
```

### Adding New Dependencies

1. Add the dependency to your code in `pkg/core/`
2. Run `make check-core-deps` - it will fail and show the new dependency
3. If the dependency is approved, add it to `scripts/core-deps-whitelist.txt`
4. Run `make check-core-deps` again to verify

### Example Output

**Success:**
```
🔍 Checking core package dependencies...
✅ All core dependencies are whitelisted
📊 Core package imports 23 external dependencies
```

**Violation:**
```
🔍 Checking core package dependencies...
❌ CORE DEPENDENCY VIOLATIONS DETECTED:
  ✗ database/sql

🚫 ARCHITECTURAL BOUNDARY VIOLATION
The core package must not import implementation dependencies directly.

�️  Database detected: Create a Repository interface in core

🤖 AI AGENT GUIDANCE:
1. DO NOT add these dependencies to the whitelist
2. DO NOT import concrete implementations in core
3. INSTEAD: Create an interface in core and use dependency injection

📋 PROPER PATTERN:
   // In core package - define interface
   type MyService interface {
       DoSomething(ctx context.Context) error
   }

   // In plugin/implementation package - implement interface
   type ConcreteMyService struct { /* ... */ }
   func (s *ConcreteMyService) DoSomething(ctx context.Context) error { /* ... */ }

   // In main/initialization - inject dependency
   service := &ConcreteMyService{}
   core.SetMyService(service)

📚 EXISTING EXAMPLES:
   - pkg/osshim/filesystem/filesystem.Interface (for file operations)
   - pkg/osshim/httpclient/client.Interface (for HTTP operations)
   - pkg/core/registry/registry.Interface (for repository access)
```

## Integration

The dependency check is automatically run as part of:
- `make verify` (full verification pipeline)
- `make check-core-deps` (standalone check)

## Maintenance

Periodically review the whitelist for:
- Unused entries (script will warn about these)
- Dependencies that should be moved to interfaces
- Opportunities to reduce core dependencies
