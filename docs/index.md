---
layout: home

hero:
  name: myCart
  text: Shopping-cart in 1 file
  tagline: Single-binary e-commerce solution with Go + SQLite + SvelteKit
  actions:
    - theme: brand
      text: Get Started
      link: /readme
    - theme: alt
      text: View on GitHub
      link: https://github.com/shurco/mycart
    - theme: alt
      text: API Documentation
      link: /swagger/
      target: _blank
      rel: noopener noreferrer

features:
  - icon: 📦
    title: Single Binary
    details: Everything embedded - admin panel, storefront, and API in one executable file
  - icon: 🗄️
    title: SQLite Database
    details: Embedded database with no external dependencies or setup required
  - icon: ⚡
    title: Go Backend
    details: Fast, reliable, and easy to deploy backend built with Go 1.26
  - icon: 🎨
    title: SvelteKit Frontends
    details: Modern admin panel and storefront built with SvelteKit and Tailwind CSS
  - icon: 🔌
    title: Complete API
    details: RESTful API with full Swagger/OpenAPI documentation
  - icon: ✅
    title: E2E Tested
    details: Comprehensive Playwright test coverage for reliability
---

## Quick Start

```bash
# Download and run
./mycart serve

# Access admin panel
open http://localhost:8080/_/

# Access storefront
open http://localhost:8080/
```

Default credentials: `user@mail.com` / `Pass123`

## Architecture

myCart is a monolithic e-commerce backend with:

- **Go 1.26** - Backend runtime
- **Fiber v3** - HTTP framework
- **SQLite** - Embedded database via modernc.org/sqlite (pure Go, no CGO)
- **SvelteKit** - Admin and storefront SPAs
- **Goose** - Database migrations

All components are embedded into a single binary using `go:embed`.

## Documentation

- **[Getting Started](/readme)** - Installation and setup guide
- **<a href="/mycart/swagger/" target="_blank" rel="noopener noreferrer">API Documentation</a>** - Complete Swagger/OpenAPI documentation
- **[E2E Test Reports](/e2e/)** - Playwright test results
- **[Customization](/customization)** - Customize your store
- **[GitHub Repository](https://github.com/shurco/mycart)** - Source code and issues
