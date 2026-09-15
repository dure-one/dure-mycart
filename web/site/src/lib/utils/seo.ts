/**
 * Utility for working with SEO meta tags
 */

export interface SEOData {
  title?: string
  keywords?: string
  description?: string
}

import { isBrowser } from './browser'

/**
 * Updates SEO meta tags on the page
 * @param seo - SEO data
 */
export function updateSEOTags(seo: SEOData): void {
  if (!isBrowser()) return

  if (seo.title) {
    document.title = seo.title
    updateMetaTag('meta[name="title"]', 'content', seo.title)
    updateMetaTag('meta[property="og:title"]', 'content', seo.title)
  }

  if (seo.keywords) {
    updateMetaTag('meta[name="keywords"]', 'content', seo.keywords)
  }

  if (seo.description) {
    updateMetaTag('meta[name="description"]', 'content', seo.description)
    updateMetaTag('meta[property="og:description"]', 'content', seo.description)
  }
}

/**
 * Updates meta tag value
 */
function updateMetaTag(selector: string, attribute: string, value: string): void {
  const element = document.querySelector(selector)
  if (element) {
    element.setAttribute(attribute, value)
  }
}

/**
 * Points the browser at the shop's own icon, where it has uploaded one.
 *
 * Not a meta tag, so it does not go through updateSEOTags: the icon a browser
 * shows comes from the element, and rewriting its href is what makes a browser
 * that already has the built-in icon ask for this one. An empty url leaves the
 * link as the build wrote it in app.html.
 */
export function updateFavicon(url: string): void {
  if (!isBrowser() || !url) return

  let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
  if (!link) {
    link = document.createElement('link')
    link.rel = 'icon'
    document.head.appendChild(link)
  }
  link.href = url
}
