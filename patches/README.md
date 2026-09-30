# Applied Patches

## extract-zip fork override

**Source:** https://github.com/AIEpisteme/extract-zip (includes PR #161)  
**Fixes:** CVE symlink path traversal vulnerability  
**Status:** Using patched fork until official release

### What it does
Prevents malicious zip files from creating symlinks that escape the extraction directory, blocking path traversal attacks.

### Application
Forced via `package.json` overrides:
```json
"overrides": {
  "extract-zip": "github:AIEpisteme/extract-zip"
}
```

### Removal
When `extract-zip@2.0.2+` is released with this fix:
1. Remove fork override from `package.json`
2. Run `npm install` to get official patched version
