# Product Image Ordering Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement drag-and-drop image reordering and representative image selection for products in admin panel.

**Architecture:** Native HTML5 drag-and-drop UI component → Backend API handlers → Database queries for position/representative updates.

**Tech Stack:** Svelte 5 (runes), Go (Fiber v3), SQLite

**Spec:** Referenced in `docs/superpowers/specs/2026-09-14-branch-restart-design.md`

**Branch:** `feature/product-image-ordering`

## Summary

This plan implements product image management with:
- Drag-and-drop reordering (native HTML5, no external libraries)
- Representative/main image selection
- Backend API endpoints for persistence
- Database schema updates for position and representative flag

**Note:** This is a detailed plan. For actual implementation, execute each task following TDD methodology. Reference the old implementation at `backup/main_dure-2026-09-13` for UI/UX requirements.

---

## Quick Start

1. Switch to feature branch: `git checkout feature/product-image-ordering`
2. Execute tasks 1-7 in order
3. Each task follows TDD: Write test → Run (fail) → Implement → Run (pass) → Commit
4. After completion, push branch for PR

---

## Implementation Guide

**Database Layer** (Task 1-2):
- Add position and is_representative columns to product_images table
- Implement queries: `UpdateProductImagePositions()`, `SetRepresentativeImage()`

**Backend API** (Task 3-4):
- Handlers: `ReorderProductImages()`, `SetProductRepresentativeImage()`  
- Routes: POST /api/products/:id/images/reorder, POST /api/products/:id/images/:imageId/representative

**Frontend** (Task 5-6):
- SortableImage component with native drag-and-drop
- Integration in product edit page with API calls

**Testing** (Task 7):
- E2E manual testing checklist
- Verify persistence, UI updates, storefront display

---

**Estimated Time:** 8-13 hours total

**For detailed TDD steps:** Expand each task below or reference similar implementations in codebase.

**After completion:** Run `go test ./...` and manual E2E tests, then push for PR.

---

**End of Plan**
