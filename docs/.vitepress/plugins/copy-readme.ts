import fs from 'fs'
import path from 'path'
import type { Plugin } from 'vite'

export function copyReadmePlugin(): Plugin {
  return {
    name: 'copy-readme',
    buildStart() {
      const rootReadme = path.resolve(__dirname, '../../../README.md')
      const docsReadme = path.resolve(__dirname, '../../readme.md')

      // Copy root README to docs/readme.md (English)
      if (fs.existsSync(rootReadme)) {
        let content = fs.readFileSync(rootReadme, 'utf-8')

        // Fix image URLs for docs context - use raw GitHub URLs
        content = content.replace(
          /\.github\/media\//g,
          'https://raw.githubusercontent.com/shurco/mycart/main/.github/media/'
        )

        fs.writeFileSync(docsReadme, content)
        console.log('[copy-readme] Copied README.md to docs/readme.md')
      } else {
        console.warn('[copy-readme] Root README.md not found')
      }
    }
  }
}
