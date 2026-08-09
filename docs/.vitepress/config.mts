import { defineConfig } from 'vitepress'
import { withMermaid } from 'vitepress-plugin-mermaid'
import { tabsMarkdownPlugin } from 'vitepress-plugin-tabs'
import { configureDiagramsPlugin } from 'vitepress-plugin-diagrams'
import llmstxt, { copyOrDownloadAsMarkdownButtons } from 'vitepress-plugin-llms'
import { withI18n } from 'vitepress-i18n'
import { plantumlMarkdownPlugin, plantumlVitePlugin } from 'vitepress-plugin-plantuml'
import { videoMarkdownPlugin } from 'vitepress-plugin-video'
import { pdfMarkdownPlugin } from 'vitepress-plugin-pdf'
import { qrcodeMarkdownPlugin } from 'vitepress-plugin-qrcode'
import { stepsMarkdownPlugin } from 'vitepress-plugin-steps'
import { collapseMarkdownPlugin } from 'vitepress-plugin-collapse'
import { markdownPlugin as markMarkdownPlugin } from 'vitepress-plugin-mark'

// https://vitepress.dev/reference/site-config
const vitePressConfig = withMermaid(defineConfig({
  title: 'myCart',
  description: 'Open source shopping-cart backend API - a single-binary e-commerce solution',
  base: '/mycart/',

  // Ignore dead links for external directories added by GitHub workflow
  ignoreDeadLinks: [
    /^\/swagger\//,
    /^\/e2e\//,
    /http:\/\/localhost/,
    /\.\.\/k8s\//
  ],

  head: [
    ['link', { rel: 'icon', href: '/mycart/favicon.ico' }]
  ],

  // Vite plugins configuration
  vite: {
    plugins: [llmstxt(), plantumlVitePlugin()]
  },

  // Markdown plugins configuration
  markdown: {
    config(md) {
      md.use(tabsMarkdownPlugin)
      md.use(configureDiagramsPlugin, {
        krokilUrl: 'https://kroki.io'
      })
      md.use(copyOrDownloadAsMarkdownButtons)
      md.use(plantumlMarkdownPlugin)
      md.use(videoMarkdownPlugin, {
        artplayer: true,
        youtube: true,
        bilibili: true,
        acfun: true
      })
      md.use(pdfMarkdownPlugin)
      md.use(qrcodeMarkdownPlugin)
      md.use(stepsMarkdownPlugin)
      md.use(collapseMarkdownPlugin)
      md.use(markMarkdownPlugin)
    },
    languageAlias: { plantuml: 'txt' }
  },

  // Mermaid configuration
  mermaid: {
    theme: 'default'
  },

  themeConfig: {
    // https://vitepress.dev/reference/default-theme-config
    nav: [
      { text: 'Docs', link: '/' },
      { text: 'API', link: '/swagger/', target: '_blank', rel: 'noopener noreferrer' },
      { text: 'E2E', link: '/e2e/', target: '_blank', rel: 'noopener noreferrer' },
      { text: 'GitHub', link: 'https://github.com/shurco/mycart' }
    ],

    sidebar: [
      {
        text: 'Guide',
        items: [
          { text: 'Introduction', link: '/' },
          { text: 'Getting Started', link: '/readme' },
          { text: 'API Reference', link: '/api' },
          { text: 'API Product Variants', link: '/api-product-variants' },
          { text: 'Customization', link: '/customization' },
          { text: 'Payment Customization', link: '/payment-customization' },
          { text: 'Migration from LiteCart', link: '/migration-from-litecart' },
          { text: 'Development on BSD', link: '/development-on-bsd' }
        ]
      }
    ],

    socialLinks: [
      { icon: 'github', link: 'https://github.com/shurco/mycart' }
    ],

    editLink: {
      pattern: 'https://github.com/shurco/mycart/edit/main/docs/:path',
      text: 'Edit this page on GitHub'
    },

    footer: {
      message: 'Released under the MIT License.',
      copyright: 'Copyright © 2024-present shurco'
    },

    search: {
      provider: 'local'
    }
  }
}))

// i18n configuration
const i18nOptions = {
  locales: ['en'],
  rootLocale: 'en'
}

export default withI18n(vitePressConfig, i18nOptions)
