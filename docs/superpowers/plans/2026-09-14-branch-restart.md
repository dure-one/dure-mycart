# Branch Restart Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Clean restart of `main_dure` from `main`, re-applying only Build Improvements (cherry-pick) and Product Image Ordering (manual re-implementation).

**Architecture:** Tag current `main_dure` as backup, reset to `main`, create parallel feature branches, apply features independently, merge sequentially to `main_dure`.

**Tech Stack:** Git, Makefile, Docker Compose

**Spec:** `docs/superpowers/specs/2026-09-14-branch-restart-design.md`

## Global Constraints

- `main` branch MUST NOT be modified
- Only one force-push allowed: resetting `main_dure` to `main`  
- All subsequent changes via normal git merges (fork-friendly)
- Backup tag `backup/main_dure-2026-09-13` must be pushed to remote before reset
- Conventional commit message format required

---

## Scope Note

This plan covers the git workflow for branch restart and cherry-picking Build Improvements. 

**Product Image Ordering** (Tasks 6-14) is complex enough to warrant its own detailed implementation plan. After completing Tasks 1-5, create a separate plan `2026-09-14-product-image-ordering.md` with full TDD cycles for that feature.

---

### Task 1: Create Backup and Reset main_dure

**Files:**
- None created (git operations only)

**Interfaces:**
- Consumes: Current `main_dure` branch, current `main` branch
- Produces: Tag `backup/main_dure-2026-09-13` on remote, `main_dure` reset to `main`

- [ ] **Step 1: Verify current branch state**

```bash
git checkout main_dure && git status
```

Expected: Clean working directory on `main_dure` branch

- [ ] **Step 2: Create backup tag locally**

```bash
git tag backup/main_dure-2026-09-13 main_dure
```

Expected: Tag created successfully

- [ ] **Step 3: Verify tag points to correct commit**

```bash
git show backup/main_dure-2026-09-13 --oneline -s
```

Expected: Shows the current HEAD of `main_dure`

- [ ] **Step 4: Push backup tag to remote**

```bash
git push origin backup/main_dure-2026-09-13
```

Expected: Tag pushed successfully to `origin`

- [ ] **Step 5: Verify tag exists on remote**

```bash
git ls-remote --tags origin | grep backup/main_dure-2026-09-13
```

Expected: Tag appears in remote tag list

- [ ] **Step 6: Reset main_dure to main**

```bash
git reset --hard main
```

Expected: `main_dure` now points to same commit as `main`

- [ ] **Step 7: Verify reset was successful**

```bash
git log --oneline --left-right main...main_dure
```

Expected: No output (branches are identical)

- [ ] **Step 8: Push reset main_dure to remote**

```bash
git push origin main_dure --force-with-lease
```

Expected: Remote `main_dure` updated successfully

---

### Task 2: Create Feature Branches

**Files:**
- None created (git operations only)

**Interfaces:**
- Consumes: Reset `main_dure` branch (from Task 1)
- Produces: Branches `feature/build-improvements` and `feature/product-image-ordering`

- [ ] **Step 1: Create build-improvements branch**

```bash
git checkout main_dure && git checkout -b feature/build-improvements
```

Expected: Created and switched to `feature/build-improvements`

- [ ] **Step 2: Return to main_dure and create product-image-ordering branch**

```bash
git checkout main_dure && git checkout -b feature/product-image-ordering
```

Expected: Created and switched to `feature/product-image-ordering`

- [ ] **Step 3: Verify both branches exist at same commit**

```bash
git log --oneline -1 feature/build-improvements && git log --oneline -1 feature/product-image-ordering
```

Expected: Both show the same commit hash

---

### Task 3: Cherry-pick Build Improvements

**Files:**
- Cherry-picked: `Makefile`, `docker-compose.yml`, build scripts

**Interfaces:**
- Consumes: Branch `feature/build-improvements`, commits from `backup/main_dure-2026-09-13`
- Produces: `feature/build-improvements` branch with 5 cherry-picked commits

- [ ] **Step 1: Switch to build-improvements branch**

```bash
git checkout feature/build-improvements
```

Expected: On `feature/build-improvements` branch

- [ ] **Step 2: Attempt bulk cherry-pick**

```bash
git cherry-pick 3fd44bc b43a413 ce03b80 9437b8c a95ee79
```

Expected: Either all commits apply cleanly OR conflicts reported

- [ ] **Step 3: If conflicts, cherry-pick individually**

If Step 2 failed:
```bash
git cherry-pick --abort
git cherry-pick 3fd44bc
# If conflicts: resolve, git add, git cherry-pick --continue
git cherry-pick b43a413
# Repeat for each commit: ce03b80, 9437b8c, a95ee79
```

Expected: All 5 commits applied successfully

- [ ] **Step 4: Verify commits applied**

```bash
git log --oneline main_dure..feature/build-improvements
```

Expected: 5 commits listed

---

### Task 4: Test Build Improvements

**Files:**
- None created (testing only)

**Interfaces:**
- Consumes: Branch `feature/build-improvements` with cherry-picked commits
- Produces: Validated build improvements

- [ ] **Step 1: Test make build**

```bash
make build
```

Expected: Build succeeds

- [ ] **Step 2: Test make test**

```bash
make test
```

Expected: Tests pass

- [ ] **Step 3: Validate docker-compose**

```bash
docker-compose config
```

Expected: Valid YAML configuration

- [ ] **Step 4: Run Go test suite**

```bash
go test ./... -count=1 -race
```

Expected: All tests pass

- [ ] **Step 5: Test frontends**

```bash
cd web/admin && bun test && cd ../..
cd web/site && bun test && cd ../..
```

Expected: All frontend tests pass

---

### Task 5: Merge Build Improvements

**Files:**
- None created (git operations only)

**Interfaces:**
- Consumes: Validated `feature/build-improvements`, `main_dure`
- Produces: `main_dure` with build improvements merged

- [ ] **Step 1: Switch to main_dure and merge**

```bash
git checkout main_dure
git merge --no-ff feature/build-improvements -m "feat: re-apply build and development improvements

- Add Makefile with test, build, and Docker commands
- Add unified Docker Compose configuration
- Add automated setup and dependency checking
- Improve OpenBSD development setup

Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>"
```

Expected: Merge completed successfully

- [ ] **Step 2: Verify build still works**

```bash
make build && make test
```

Expected: Build and tests pass

- [ ] **Step 3: Push to remote**

```bash
git push origin main_dure
```

Expected: Push successful

---

### Task 6: Create Product Image Ordering Plan

**Note:** Product Image Ordering is complex enough for its own detailed plan.

- [ ] **Step 1: Create separate plan file**

Create `docs/superpowers/plans/2026-09-14-product-image-ordering.md` with full TDD cycles for:
- Backend database queries
- Backend handlers
- Route registration  
- Frontend @dnd-kit integration
- SortableImage component
- Product edit page integration
- E2E testing

- [ ] **Step 2: Execute that plan on feature/product-image-ordering branch**

Use `superpowers:subagent-driven-development` to execute the Product Image Ordering plan

---

### Task 7: Merge Product Image Ordering

**Files:**
- None created (git operations only)

**Interfaces:**
- Consumes: Completed `feature/product-image-ordering` branch, `main_dure`
- Produces: `main_dure` with both features

- [ ] **Step 1: Ensure product-image-ordering is complete and tested**

Verify all E2E tests passed per the separate Product Image Ordering plan

- [ ] **Step 2: Switch to main_dure and merge**

```bash
git checkout main_dure
git merge --no-ff feature/product-image-ordering -m "feat: implement product image ordering with drag-and-drop

- Add drag-and-drop image reordering in admin panel
- Add representative image selection
- Add SortableImage component using @dnd-kit
- Add product placeholder images
- Implement backend handlers and database queries

Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>"
```

Expected: Merge completed successfully

- [ ] **Step 3: Run full test suite**

```bash
make test && make build
```

Expected: All tests pass, build succeeds

- [ ] **Step 4: Push to remote**

```bash
git push origin main_dure
```

Expected: Push successful

---

### Task 8: Final Verification

**Files:**
- Optional: `docs/branch-restart-summary-2026-09-14.md`

**Interfaces:**
- Consumes: Completed `main_dure` with both features
- Produces: Verified success

- [ ] **Step 1: Verify branch states**

```bash
git log --oneline --graph main_dure -10
```

Expected: Shows 2 merge commits (build improvements + product image ordering)

- [ ] **Step 2: Verify main unchanged**

```bash
git log --oneline main -1
```

Expected: Still shows original main commit

- [ ] **Step 3: Count commits difference**

```bash
git rev-list --count main..main_dure
git rev-list --count main_dure..main
```

Expected: ~7-15 commits in main_dure, 0 in main

- [ ] **Step 4: Verify backup tag exists**

```bash
git tag -l "backup/*" && git ls-remote --tags origin | grep backup
```

Expected: Backup tag exists locally and remotely

- [ ] **Step 5: Final smoke test**

```bash
go run ./cmd serve
```

Expected: Server starts, manually test build tools and product image ordering work

- [ ] **Step 6: Document success**

Create summary documenting what was done, commits applied, tests run

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-09-14-branch-restart.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?
