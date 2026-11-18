import { sveltekit } from '@sveltejs/kit/vite'
import { defineConfig } from 'vite'
import tailwindcss from '@tailwindcss/vite'
import adapter from '@sveltejs/adapter-static'
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte'

export default defineConfig({
  plugins: [
    sveltekit({
      preprocess: vitePreprocess(),
      adapter: adapter({
        pages: 'build',
        assets: 'build',
        fallback: 'index.html',
        precompress: false,
        strict: false
      }),
      alias: {
        $lib: './src/lib',
        '@': './src'
      }
    }),
    tailwindcss()
  ],
  server: {
    port: 5273,
    proxy: {
      '/api': {
        target: 'http://localhost:8080/',
        changeOrigin: true
      },
      '/uploads': {
        target: 'http://localhost:8080/',
        changeOrigin: true
      },
      '^/cart/payment': {
        target: 'http://localhost:8080/',
        changeOrigin: true,
        ws: true
      }
    }
  }
})
