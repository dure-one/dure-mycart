import fs from 'fs'
import path from 'path'

// Must run before VitePress globs docs/ for its page list (i.e. called
// eagerly from config.mts, not from a Vite plugin hook like buildStart -
// those run too late and the generated docs/readme.md silently never
// becomes a page).
export function copyReadme(): void {
  const rootReadme = path.resolve(__dirname, '../../../README.md')
  const docsReadme = path.resolve(__dirname, '../../readme.md')

  // Copy root README to docs/readme.md (English). README.md already uses
  // raw.githubusercontent.com URLs for every .github/media image, so no
  // image-URL rewriting is needed here.
  if (fs.existsSync(rootReadme)) {
    let content = fs.readFileSync(rootReadme, 'utf-8')

    // Fix internal doc links for docs context - strip the redundant
    // "docs/" prefix since this file already lives inside docs/
    content = content.replace(
      /\]\(\.\/docs\//g,
      '](./'
    )

    fs.writeFileSync(docsReadme, content)
    console.log('[copy-readme] Copied README.md to docs/readme.md')
  } else {
    console.warn('[copy-readme] Root README.md not found')
  }
}
