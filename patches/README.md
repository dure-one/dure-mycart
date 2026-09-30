# Applied Patches

## extract-zip-symlink-security.patch

**Source:** https://github.com/max-mapper/extract-zip/pull/161  
**Fixes:** CVE symlink path traversal vulnerability  
**Status:** Unreleased (PR pending)

### What it does
Prevents malicious zip files from creating symlinks that escape the extraction directory, blocking path traversal attacks.

### Application
Auto-applied via `postinstall` hooks:
- `scripts/patch-extract-zip.js` applies patch to all `extract-zip` installations
- Runs after `npm install` in root, web/admin, and web/site

### Removal
When `extract-zip@2.0.2+` is released with this fix:
1. Remove from `package.json` overrides (if used)
2. Remove `scripts/patch-extract-zip.js`
3. Remove postinstall hook calls
4. Delete this patch file
5. Run `npm install` to get official patched version
