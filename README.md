
# dure-mycart

This is fork of shurco/mycart.

- shurco/mycart:main -> nikescar/mycart:main
- nikescar/mycart:main_dure -> dure-one/dure-mycart:main

Two different trees.

## Quick Start

For first-time setup on a new machine:

```bash
make setup
```

This automatically installs all prerequisites (npm dependencies, Go modules, sqlc code generation).

Then start the development server:

```bash
make dev
```

## Documentation

- **[SETUP.md](SETUP.md)** - Complete setup guide and troubleshooting
- Run `make help` - See all available commands

## Common Commands

```bash
make setup         # Install all prerequisites (run this first!)
make deps-check    # Verify dependencies are installed
make dev           # Start development server
make test          # Run backend tests
make e2e-all       # Run E2E tests
make build-all     # Build frontend applications
```

## Prerequisites

- Node.js (v18+) and npm
- Go (v1.21+)
- sqlc (v1.31+)

See [SETUP.md](SETUP.md) for detailed installation instructions.
