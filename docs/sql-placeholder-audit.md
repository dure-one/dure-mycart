# SQL Placeholder Audit Report
## PostgreSQL Compatibility Issues

Generated: $(date)

### Summary
Found hardcoded SQLite-style `?` placeholders in **3 critical files** that break PostgreSQL compatibility.

---

## 1. internal/goosemigration/queries/setting.go
**Status:** 🔴 CRITICAL - Used during authentication

**Issues Found:**
- Line 230: `GetSettingByGroup` - IN clause with dynamic placeholders
- Line 303-306: `UpdateSettingByGroup` - UPSERT statement  
- Line 342: `UpdatePassword` - UPDATE statement

**Impact:** These queries are used for JWT settings retrieval during login

**Fix Required:**
```go
// Add conditional placeholder building
func buildQueryPlaceholder(index int) string {
    if DBType() == "postgres" || DBType() == "postgresql" {
        return fmt.Sprintf("$%d", index)
    }
    return "?"
}
```

---

## 2. internal/goosemigration/queries/pages.go  
**Status:** 🟡 MEDIUM - Public-facing content

**Issues Found:**
- Line 120: `loadPageSeo` - SELECT by ID
- Line 178-186: `ListPages` - Dynamic LIMIT/OFFSET
- Line 522: `UpdatePage` - UPDATE seo field

**Impact:** Page management and display functionality

**Fix Required:**
Use conditional query building based on DBType()

---

## 3. internal/goosemigration/queries/products.go
**Status:** 🟡 MEDIUM - E-commerce core

**Issues Found (19 instances):**
- Line 202: Product SELECT by ID
- Line 208: Product SELECT by slug
- Line 307: Product images query
- Line 335: Product options query  
- Line 365: Product variants query
- Lines 533, 587-588, 715, 760, 852-853, 920, 980, 1042, 1067, 1069-1070, 1271, 1339: Various product operations

**Impact:** Product display, cart operations, digital goods delivery

**Fix Required:**
Extensive conditional query building or migrate to sqlc

---

## Already Fixed ✅

### internal/store/settings.go
- ✅ Added `buildPlaceholders()` helper function
- ✅ Added `buildUpsertQuery()` for UPSERT operations
- ✅ Fixed `GetSettingByGroup`, `UpdateSettingByGroup`, `UpdatePassword`
- ✅ Added `InitStoreWithType()` to track database type

### internal/app.go
- ✅ Updated to call `store.InitStoreWithType()` with database type

---

## Recommended Fix Strategy

### Phase 1: High Priority (Blocking login) ⚡
1. Fix `internal/goosemigration/queries/setting.go`
   - Same patterns as already fixed in `internal/store/settings.go`
   - Copy helper functions from store package

### Phase 2: Medium Priority (User-facing features) 📋
2. Fix `internal/goosemigration/queries/pages.go`
3. Fix `internal/goosemigration/queries/products.go`

### Phase 3: Long-term (Code quality) 🔄
4. Migrate remaining queries to sqlc for type safety
5. Add integration tests for PostgreSQL
6. Document database-agnostic query patterns

---

## Helper Function Template

Add to `internal/goosemigration/queries/queries.go`:

```go
// BuildPlaceholder returns database-specific parameter placeholder
func BuildPlaceholder(index int) string {
    if DBType() == "postgres" || DBType() == "postgresql" {
        return fmt.Sprintf("$%d", index)
    }
    return "?"
}

// BuildPlaceholders returns comma-separated placeholders for IN clauses
func BuildPlaceholders(count int) string {
    if count == 0 {
        return ""
    }
    
    if DBType() == "postgres" || DBType() == "postgresql" {
        parts := make([]string, count)
        for i := 0; i < count; i++ {
            parts[i] = fmt.Sprintf("$%d", i+1)
        }
        return strings.Join(parts, ", ")
    }
    
    return strings.Repeat("?, ", count-1) + "?"
}
```

---

## Testing Checklist

After fixes:
- [ ] Test login with PostgreSQL (JWT settings retrieval)  
- [ ] Test page creation/update with PostgreSQL
- [ ] Test product CRUD with PostgreSQL
- [ ] Test cart operations with PostgreSQL
- [ ] Run existing integration tests with DATABASE_URL set to PostgreSQL
- [ ] Verify SQLite still works (backward compatibility)

