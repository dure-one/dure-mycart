import { redirect } from '@sveltejs/kit'
import { browser } from '$app/environment'
import { base } from '$app/paths'
import type { LayoutLoad } from './$types'

// Cache keys and TTLs
const INSTALL_STATUS_KEY = 'mycart:install_status'
const INSTALL_STATUS_TTL = 5 * 60 * 1000 // 5 minutes
const VERSION_KEY = 'mycart:version'
const VERSION_TTL = 60 * 60 * 1000 // 1 hour

interface CachedData<T> {
  data: T
  timestamp: number
}

function getCached<T>(key: string, ttl: number): T | null {
  if (!browser) return null
  try {
    const cached = localStorage.getItem(key)
    if (!cached) return null

    const parsed: CachedData<T> = JSON.parse(cached)
    if (Date.now() - parsed.timestamp > ttl) {
      localStorage.removeItem(key)
      return null
    }
    return parsed.data
  } catch {
    return null
  }
}

function setCache<T>(key: string, data: T): void {
  if (!browser) return
  try {
    const cached: CachedData<T> = {
      data,
      timestamp: Date.now()
    }
    localStorage.setItem(key, JSON.stringify(cached))
  } catch {
    // Ignore storage errors
  }
}

function clearCache(key: string): void {
  if (!browser) return
  try {
    localStorage.removeItem(key)
  } catch {
    // Ignore
  }
}

export const load: LayoutLoad = async ({ url, fetch }) => {
  const pathname = url.pathname

  // Allow signin and install pages without authentication
  if (pathname.endsWith('/signin') || pathname.endsWith('/install')) {
    return {}
  }

  if (!browser) {
    return {}
  }

  // Check install status (cached)
  let installed = getCached<boolean>(INSTALL_STATUS_KEY, INSTALL_STATUS_TTL)
  if (installed === null) {
    try {
      const statusResponse = await fetch('/api/install/status', {
        method: 'GET',
        credentials: 'include'
      })

      if (statusResponse.ok) {
        const statusData = await statusResponse.json()
        installed = statusData?.result?.installed ?? false
        setCache(INSTALL_STATUS_KEY, installed)

        if (!installed) {
          throw redirect(302, `${base}/install`)
        }
      }
    } catch (error) {
      if (error && typeof error === 'object' && 'status' in error && error.status === 302) {
        throw error
      }
    }
  } else if (!installed) {
    throw redirect(302, `${base}/install`)
  }

  // Check authentication via version endpoint (cached)
  let isAuthenticated = getCached<boolean>(VERSION_KEY, VERSION_TTL)
  if (isAuthenticated === null) {
    try {
      const versionResponse = await fetch('/api/_/version', {
        method: 'GET',
        credentials: 'include'
      })

      if (versionResponse.ok) {
        const versionData = await versionResponse.json()
        isAuthenticated = versionData?.success ?? false
        setCache(VERSION_KEY, isAuthenticated)
      } else if (versionResponse.status === 400 || versionResponse.status === 401 || versionResponse.status === 403) {
        isAuthenticated = false
        clearCache(VERSION_KEY)
        throw redirect(302, `${base}/signin`)
      }
    } catch (error) {
      if (error && typeof error === 'object' && 'status' in error && error.status === 302) {
        throw error
      }
      clearCache(VERSION_KEY)
      throw redirect(302, `${base}/signin`)
    }
  }

  if (!isAuthenticated) {
    throw redirect(302, `${base}/signin`)
  }

  return {}
}
