---
title: myCart
titleTemplate: Open source shopping-cart backend API
layout: home

hero:
  text: Shopping-cart in 1 file
  tagline: Single-binary e-commerce solution with Go + SQLite + SvelteKit
  actions:
    - text: Get Started
      link: /readme
    - theme: alt
      text: View on GitHub
      link: https://github.com/shurco/mycart
    - theme: alt
      text: API Documentation
      link: /swagger
---

## Features

- **Single Binary** - Everything embedded: admin panel, storefront, API
- **SQLite Database** - Embedded database, no external dependencies
- **Go Backend** - Fast, reliable, easy to deploy
- **SvelteKit Frontends** - Modern admin panel and storefront
- **Complete API** - RESTful API with Swagger documentation
- **E2E Tested** - Comprehensive Playwright test coverage

## Quick Start

```bash
# Download and run
./mycart serve

# Access admin panel
open http://localhost:8080/_/

# Access storefront
open http://localhost:8080/
```

## Documentation

- **[API Documentation](/swagger)** - Complete Swagger/OpenAPI documentation
- **[E2E Test Reports](/e2e)** - Playwright test results
- **[GitHub Repository](https://github.com/shurco/mycart)** - Source code and issues

## Architecture

myCart is a monolithic e-commerce backend with:

- **Go 1.26** - Backend runtime
- **Fiber v3** - HTTP framework
- **SQLite** - Embedded database via modernc.org/sqlite
- **SvelteKit** - Admin and storefront SPAs
- **Goose** - Database migrations

All components are embedded into a single binary using `go:embed`.
