import { defineConfig } from 'vitepress'

// https://vitepress.dev/reference/site-config
export default defineConfig({
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
})
