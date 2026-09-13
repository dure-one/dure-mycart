# Branch Restart Design: Fresh Start from Main

**Date:** 2026-09-14  
**Author:** Claude Sonnet 4.5  
**Status:** Design Approved  
**Classification:** Architectural

---

## 1. Overview & Goals

### Objective

Clean restart of `main_dure` from `main` with only two feature sets re-applied via parallel feature branches.

### What We're Doing

1. **Preserve current work** - Tag `main_dure` as `backup/main_dure-2026-09-13`
2. **Reset main_dure** - Make it identical to `main` (discarding 164 other commits)
3. **Re-apply features** - Cherry-pick Build Improvements + Re-implement Product Image Ordering
4. **Maintain quality** - Full E2E testing before merging

### What We're NOT Doing

- ❌ Not modifying `main` branch
- ❌ Not preserving the 164 other commits in `main_dure`
- ❌ Not force-pushing to any remote branches (fork-safe after initial reset)

### End State

- `main` - unchanged
- `main_dure` - synced with main + 2 features only
- `backup/main_dure-2026-09-13` - safety backup tag
- Feature branches - can be deleted after merge or kept for reference

---

## 2. Commit Identification

### Feature 1: Product Image Ordering

**Implementation Strategy:** Manual re-implementation (not cherry-pick)

**Reason:** Base system has changed significantly (database refactoring, sqlc migration). Cherry-picking would break due to missing infrastructure dependencies.

**Reference commits** (use as specification, not code):

```
62dcb75 feat(store): add image position and rep image queries
a2913ad feat(handlers): add ReorderProductImages handler
eeaf055 feat(handlers): add GetProductRepresentativeImage handler
a1f14a4 feat(routes): register image reorder and rep image routes
14bc659 feat(assets): add product placeholder image
40c4704 feat(deps): add @dnd-kit for drag-and-drop image reordering
7b4dd95 feat(ui): add SortableImage component for drag-and-drop
97a6f7e feat(admin): add drag-and-drop image reordering to product edit
8f4a731 fix(db): sort product images by position not id
35383bf feat(db): add ReorderProductImages and GetProductRepresentativeImage
fa70957 chore: remove debug logging from product image ordering
ca6a43d chore: remove debug logging from product image ordering
```

**Features to implement:**
- Drag-and-drop image reordering in admin panel
- Representative image selection
- SortableImage component using @dnd-kit
- Product placeholder images
- Backend handlers and database queries

### Feature 2: Build & Development Improvements

**Implementation Strategy:** Cherry-pick original commits

**Reason:** Build tooling is independent of application code, should apply cleanly.

**Commits to cherry-pick** (in chronological order):

```
3fd44bc feat(build): add Makefile with test, build, and Docker commands
b43a413 feat(docker): add unified Docker Compose configuration
ce03b80 feat(build): add automated setup and dependency checking
9437b8c feat(build): add automated setup and dependency checking
a95ee79 chore: improve OpenBSD development setup and clean dependencies
```

**Skip merge commits:**
- `df2dd1e` Merge branch 'chore/openbsd-dev-improvements' into main_dure

---

## 3. Branch Strategy & Workflow

### Phase 1: Backup & Reset (Safety First)

```bash
# 1. Create safety backup
git tag backup/main_dure-2026-09-13 main_dure
git push origin backup/main_dure-2026-09-13

# 2. Reset main_dure to match main
git checkout main_dure
git reset --hard main
git push origin main_dure --force-with-lease
```

**Safety:** The tag preserves all current work. Can restore anytime with:
```bash
git reset --hard backup/main_dure-2026-09-13
```

### Phase 2: Create Feature Branches (Parallel)

```bash
# Both branches start from the same point (synced main_dure)
git checkout main_dure
git checkout -b feature/product-image-ordering
git checkout main_dure
git checkout -b feature/build-improvements
```

### Phase 3A: Cherry-Pick Build Improvements

```bash
git checkout feature/build-improvements
git cherry-pick 3fd44bc b43a413 ce03b80 9437b8c a95ee79
```

**If conflicts occur:**
```bash
# Resolve conflicts in files
git add <resolved-files>
git cherry-pick --continue
```

### Phase 3B: Re-implement Product Image Ordering

```bash
git checkout feature/product-image-ordering

# Reference the old commits for requirements:
git show backup/main_dure-2026-09-13:<file-path> > /tmp/reference.txt
git diff backup/main_dure-2026-09-13~12..backup/main_dure-2026-09-13 -- <path>
```

**Implementation Guide:**

1. **Review reference commits** for WHAT was built (requirements), not HOW
2. **Adapt to current infrastructure:**
   - Use current database layer patterns in `internal/queries/`
   - Follow current handler patterns in `internal/handlers/`
   - Match current frontend patterns in `web/admin/`
3. **Implement incrementally** with test-driven development
4. **Create meaningful commits** following conventional commit format

**Implementation order:**
1. Backend: Database queries (image position, representative image)
2. Backend: Handlers (ReorderProductImages, GetProductRepresentativeImage)
3. Backend: Routes registration
4. Frontend: Install @dnd-kit dependency
5. Frontend: SortableImage component
6. Frontend: Integrate into product edit page
7. Assets: Add placeholder image
8. Testing: Unit, integration, E2E

### Phase 4: Merge to main_dure (Sequential)

```bash
# Merge build improvements first (lower risk)
git checkout main_dure
git merge --no-ff feature/build-improvements -m "feat: re-apply build and development improvements

- Add Makefile with test, build, and Docker commands
- Add unified Docker Compose configuration
- Add automated setup and dependency checking
- Improve OpenBSD development setup

Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>"

# Test after first merge
make test
make build

# Then merge product image ordering
git merge --no-ff feature/product-image-ordering -m "feat: implement product image ordering with drag-and-drop

- Add drag-and-drop image reordering in admin panel
- Add representative image selection
- Add SortableImage component using @dnd-kit
- Add product placeholder images
- Implement backend handlers and database queries

Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>"

# Push to remote
git push origin main_dure
```

**Using `--no-ff`:**
- Preserves feature branch history
- Creates explicit merge commits
- Easy to revert entire feature: `git revert -m 1 <merge-commit>`

---

## 4. Testing Strategy

### Build Improvements Testing

**After cherry-picking to `feature/build-improvements`:**

```bash
# 1. Test Makefile targets
make test
make build

# 2. Test Docker Compose
docker-compose up -d
docker-compose down

# 3. Test OpenBSD-specific improvements (if on OpenBSD)
# Verify dependencies install correctly
# Check automated setup scripts

# 4. Run existing test suite
go test ./... -count=1 -race
cd web/admin && bun test
cd web/site && bun test
```

**Pass Criteria:**
- ✅ All Makefile commands succeed
- ✅ Docker Compose starts/stops cleanly
- ✅ No regressions in existing tests
- ✅ OpenBSD setup scripts run without errors (if applicable)

---

### Product Image Ordering Testing

**During re-implementation on `feature/product-image-ordering`:**

#### 1. Unit Tests (Write as you implement - TDD)

```bash
# Backend unit tests
go test ./internal/handlers/... -run ProductImage -v
go test ./internal/queries/... -run ProductImage -v

# Frontend component tests
cd web/admin
bun test -- SortableImage
```

#### 2. Backend Integration Tests

```bash
# Test API endpoints
go test ./internal/handlers/... -run TestReorderProductImages -v
go test ./internal/handlers/... -run TestGetProductRepresentativeImage -v
```

#### 3. Frontend Integration Tests

```bash
cd web/admin
bun test -- "product edit"
```

#### 4. Full E2E Testing (CRITICAL)

```bash
# Start server
go run ./cmd serve

# Manual testing checklist:
# 1. Login to admin panel (/_/)
# 2. Navigate to Products → Edit existing product with images
# 3. Test drag-and-drop:
#    - Drag image to new position
#    - Verify visual feedback during drag
#    - Verify order updates in UI
# 4. Test representative image:
#    - Click to set representative image
#    - Verify visual indicator
# 5. Save product
# 6. Reload page - verify order persists
# 7. Visit storefront
# 8. Verify product images show in correct order
# 9. Verify representative image displays
# 10. Check browser console - no errors
```

**Pass Criteria:**
- ✅ Images can be reordered via drag-and-drop
- ✅ Representative image selection works
- ✅ Order persists after save/reload
- ✅ Storefront displays images in correct order
- ✅ Representative image shows on storefront
- ✅ No console errors or warnings
- ✅ All unit and integration tests pass

---

### Integration Testing (After merging both to main_dure)

```bash
# Full system test
git checkout main_dure
make build
make test
go run ./cmd serve

# Verify both features work together:
# 1. Build improvements (Makefile, Docker)
# 2. Product image ordering functionality
# 3. No interference between features
```

**Pass Criteria:**
- ✅ All tests pass
- ✅ Application builds and runs
- ✅ Both features functional
- ✅ No regression in existing features

---

## 5. Rollback & Safety

### Safety Mechanisms

**1. Backup Tag (Created in Phase 1)**

```bash
# Restore entire main_dure anytime:
git checkout main_dure
git reset --hard backup/main_dure-2026-09-13
git push origin main_dure --force-with-lease
```

**2. Feature Branch Isolation**

- Each feature lives in its own branch
- Can delete/recreate without affecting main_dure
- Can test independently before merging
- Can cherry-pick individual commits if needed

**3. No-FF Merges**

- Using `--no-ff` preserves feature branch history
- Creates explicit merge commits
- Easy to identify feature boundaries in git log
- Simple revert: `git revert -m 1 <merge-commit>`

### Rollback Scenarios

**Scenario 1: Build Improvements has conflicts during cherry-pick**

```bash
# Option A: Resolve conflicts
git cherry-pick --abort
# Cherry-pick one commit at a time
git cherry-pick 3fd44bc
# ... resolve, then continue ...
git cherry-pick b43a413
# etc.

# Option B: Start over
git checkout main_dure
git branch -D feature/build-improvements
git checkout -b feature/build-improvements
# Re-attempt with different approach
```

**Scenario 2: Product Image Ordering implementation fails**

```bash
# Delete branch and start fresh
git checkout main_dure
git branch -D feature/product-image-ordering
git checkout -b feature/product-image-ordering

# Try different implementation approach
# Reference old code differently
# Break into smaller commits
```

**Scenario 3: Need to undo a merge to main_dure**

```bash
git checkout main_dure
git log --oneline --graph  # Find merge commit SHA

# Revert the merge (keeps history)
git revert -m 1 <merge-sha>
git push origin main_dure

# Or hard reset (rewrites history - use with caution)
git reset --hard HEAD~1
git push origin main_dure --force-with-lease
```

**Scenario 4: Complete disaster recovery**

```bash
# Restore from backup tag
git checkout main_dure
git reset --hard backup/main_dure-2026-09-13
git push origin main_dure --force-with-lease

# All work preserved in backup tag
# Can re-attempt entire process
```

### Fork Safety

- ✅ No force-push to `main` (never touched)
- ✅ Force-push to `main_dure` only once (during initial reset in Phase 1)
- ✅ After reset, all changes via normal merges (fork-friendly)
- ✅ Forks can `git pull` cleanly after initial reset
- ✅ Clear communication: "main_dure was reset to main on 2026-09-14"

**For Fork Maintainers:**

After the reset, forks should:
```bash
# Backup their fork's main_dure
git checkout main_dure
git tag my-backup-2026-09-13
git push origin my-backup-2026-09-13

# Sync with upstream
git fetch upstream
git reset --hard upstream/main_dure
git push origin main_dure --force-with-lease
```

---

## 6. Success Criteria

### Feature-Level Success

**Build Improvements:**
- ✅ Makefile commands work (`make test`, `make build`)
- ✅ Docker Compose starts/stops cleanly
- ✅ OpenBSD setup scripts run without errors
- ✅ All existing tests pass
- ✅ No regression in build process

**Product Image Ordering:**
- ✅ Drag-and-drop reordering works in admin UI
- ✅ Visual feedback during drag operation
- ✅ Representative image selection functional
- ✅ Image order persists to database
- ✅ Storefront displays images in correct order
- ✅ Storefront shows representative image
- ✅ No console errors or warnings
- ✅ Unit tests pass (backend + frontend)
- ✅ Integration tests pass
- ✅ E2E manual testing passed

### Repository-Level Success

**Branch State:**
- ✅ `main` - unchanged from start
- ✅ `main_dure` - synced with main + 2 features only
- ✅ `backup/main_dure-2026-09-13` - tag exists and pushed to remote
- ✅ Feature branches merged cleanly (can be deleted or kept)

**Quality Gates:**
- ✅ `go test ./... -count=1 -race` passes
- ✅ Frontend tests pass (`bun test` in web/admin and web/site)
- ✅ E2E manual testing completed and documented
- ✅ No regression in existing functionality
- ✅ Code follows existing patterns in current `main`
- ✅ Conventional commit messages used

### Process Success

- ✅ No force-push to `main` branch
- ✅ Clean git history (meaningful commit messages)
- ✅ Documentation updated (if needed)
- ✅ Fork-friendly (upstream can pull cleanly after initial reset)
- ✅ All rollback mechanisms tested and documented

---

## 7. Risk Analysis

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Cherry-pick conflicts in build improvements | Medium | Low | Cherry-pick one at a time, resolve incrementally |
| Product image ordering doesn't match current DB schema | High | High | Manual re-implementation, adapt to current schema |
| Breaking existing product functionality | Medium | High | Comprehensive testing, test on separate branch first |
| Fork maintainers miss the reset notice | Medium | Medium | Clear communication, document in CHANGELOG |
| Backup tag gets deleted accidentally | Low | High | Push tag to remote immediately, document restoration process |
| E2E testing incomplete | Medium | High | Detailed testing checklist, document test cases |

---

## Appendix: Reference Commands

### View Old Implementation

```bash
# Show full diff of product image ordering feature
git diff backup/main_dure-2026-09-13~12..backup/main_dure-2026-09-13 -- web/admin/src/routes/products/

# Show specific file from old implementation
git show backup/main_dure-2026-09-13:web/admin/src/lib/components/SortableImage.svelte

# List all changed files in the feature
git diff --name-only backup/main_dure-2026-09-13~12..backup/main_dure-2026-09-13
```

### Verify Tag Exists

```bash
# Local tags
git tag -l "backup/*"

# Remote tags
git ls-remote --tags origin | grep backup
```

### Check Branch Sync Status

```bash
# Compare main_dure with main
git log --oneline --left-right main...main_dure

# Count commits difference
git rev-list --count main..main_dure
git rev-list --count main_dure..main
```

---

**End of Design Document**
