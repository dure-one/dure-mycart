# Quick Setup Guide

This guide helps you set up the mycart project on a new platform/machine.

## Prerequisites

Make sure you have these installed on your system:

- **Node.js** (v18+) and npm
- **Go** (v1.21+)
- **sqlc** (v1.31+) - Install: `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`

## One-Command Setup

For a fresh clone, run:

```bash
make setup
```

This will automatically:
- ✅ Install npm dependencies (including patchright)
- ✅ Download Go module dependencies
- ✅ Generate sqlc database code
- ✅ Apply OpenBSD-specific patches (if on OpenBSD)

## Verify Installation

Check all dependencies are correctly installed:

```bash
make deps-check
```

Expected output:
```
🔍 Checking dependencies...

Node.js: v24.x.x
npm: 11.x.x
Go: go version go1.26.x ...
sqlc: v1.31.x
patchright: ✓ installed
Go modules: ✓ installed
sqlc generated: ✓ generated
```

## Development Workflow

### First Time Setup

```bash
# 1. Clone the repository
git clone <repo-url>
cd mycart

# 2. Run setup (installs everything)
make setup

# 3. Start development server
make dev
```

### Running Tests

```bash
# Backend tests
make test              # All tests with SQLite
make test-unit         # Unit tests only
make test-postgres     # Integration tests with PostgreSQL

# Frontend E2E tests (auto-runs setup if needed)
make e2e-all
```

### Building

```bash
# Build admin panel
make build-admin

# Build storefront
make build-site

# Build both
make build-all
```

## Troubleshooting

### Issue: "patchright: not found"

**Solution:**
```bash
make setup
```

### Issue: "package github.com/xxx/xxx: no matching versions"

**Solution:**
```bash
go mod tidy
sqlc generate
```

Or just run:
```bash
make setup
```

### Issue: OpenBSD-specific binary issues

The `postinstall` script automatically patches patchright for OpenBSD. If you encounter issues:

1. Check `scripts/patch-patchright-openbsd.js` ran successfully
2. Re-run: `npm run postinstall`

### Issue: sqlc-generated code missing

**Solution:**
```bash
sqlc generate
```

Or:
```bash
make sqlc
```

## What `make setup` Does

1. **npm dependencies**: Installs all Node.js packages with `--legacy-peer-deps` flag (required for some vitepress plugins)
2. **Go modules**: Downloads and verifies all Go dependencies
3. **sqlc generation**: Generates type-safe Go code from SQL queries
4. **Platform patches**: Applies OpenBSD-specific patches to patchright (if applicable)

## NPM Scripts

The package.json includes automated setup scripts:

- `npm run setup` - Full npm install with legacy peer deps
- `npm run setup:go` - Install Go deps and generate sqlc code
- `npm install` - Automatically runs `postinstall` → patches + Go setup

## Common Make Targets

```bash
make help          # Show all available targets
make setup         # Install all prerequisites
make deps-check    # Verify dependencies
make dev           # Start development server
make test          # Run backend tests
make e2e-all       # Run E2E tests
make build-all     # Build frontend apps
```

## Platform-Specific Notes

### OpenBSD
- Patchright patches are applied automatically
- Some npm packages may require `--legacy-peer-deps`
- All handled by `make setup`

### Linux/macOS
- Standard setup should work without modifications
- Run `make setup` on first clone
