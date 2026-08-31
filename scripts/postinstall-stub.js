#!/usr/bin/env node
/**
 * Postinstall stub - used when main postinstall script is not available
 * The real script (postinstall-openbsd-natives.js) only runs on BSD anyway,
 * so this stub is safe for all other platforms (Alpine Docker, Ubuntu CI, etc.)
 */
console.log('ℹ️  Postinstall stub - skipping native module patches (not needed on this platform)');
process.exit(0);
