#!/usr/bin/env node
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

// Find extract-zip in node_modules (handles nested dependencies)
const findExtractZip = (startDir) => {
  const nodeModulesPath = path.join(startDir, 'node_modules', 'extract-zip');
  if (fs.existsSync(nodeModulesPath)) {
    return nodeModulesPath;
  }

  // Check in web/admin and web/site
  const adminPath = path.join(startDir, 'web', 'admin', 'node_modules', 'extract-zip');
  if (fs.existsSync(adminPath)) return adminPath;

  const sitePath = path.join(startDir, 'web', 'site', 'node_modules', 'extract-zip');
  if (fs.existsSync(sitePath)) return sitePath;

  return null;
};

const applyPatch = (extractZipDir) => {
  const indexPath = path.join(extractZipDir, 'index.js');
  const patchPath = path.join(__dirname, '..', 'patches', 'extract-zip-symlink-security.patch');

  if (!fs.existsSync(indexPath)) {
    console.warn(`⚠️  extract-zip not found at ${indexPath}`);
    return false;
  }

  // Check if already patched
  const content = fs.readFileSync(indexPath, 'utf8');
  if (content.includes('Out of bound symlink target')) {
    console.log('✓ extract-zip already patched (symlink security fix)');
    return true;
  }

  try {
    // Apply patch
    execSync(`patch -p1 -d "${extractZipDir}" < "${patchPath}"`, {
      stdio: 'inherit',
      encoding: 'utf8'
    });
    console.log('✓ Applied extract-zip symlink security patch (PR #161)');
    return true;
  } catch (err) {
    console.error('✗ Failed to apply extract-zip patch:', err.message);
    return false;
  }
};

// Apply to all instances
const projectRoot = path.resolve(__dirname, '..');
const locations = [
  findExtractZip(projectRoot),
  path.join(projectRoot, 'web', 'admin', 'node_modules', 'extract-zip'),
  path.join(projectRoot, 'web', 'site', 'node_modules', 'extract-zip'),
];

let patched = 0;
locations.forEach(dir => {
  if (dir && fs.existsSync(dir)) {
    if (applyPatch(dir)) patched++;
  }
});

if (patched === 0) {
  console.warn('⚠️  No extract-zip installations found to patch');
}
