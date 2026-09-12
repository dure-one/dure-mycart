import fs from 'node:fs'
import path from 'node:path'

/**
 * HTTP client for the parts of the API the suite needs for setup.
 *
 * Some fixtures are impractical to build through the UI — a free product, a
 * product that sells out while it is already in a cart, the order list — so
 * they are built through the same endpoints the admin panel itself calls.
 */

/** Admin account created by `e2e/global-setup.ts`. */
export const ADMIN_EMAIL = 'admin@example.com'
export const ADMIN_PASSWORD = 'test1234'

export interface ProductInput {
  name: string
  slug: string
  brief?: string
  description?: string
  /** Price in cents: 0 makes the product free. */
  amount?: number
  quantity?: number
  sku?: string
  /** False leaves the product off the storefront. Defaults to true here. */
  active?: boolean
  digital?: { type: string }
}

export interface Product extends ProductInput {
  id: string
  active: boolean
}

export interface CartSummary {
  id: string
  email: string
  amount_total: number
  currency: string
  payment_status: string
  payment_system: string
}

export interface InstallStatus {
  installed: boolean
  database: {
    driver: string
    dsn: string
    source: string
    locked: boolean
  }
}

export interface DatabaseChoice {
  driver: 'sqlite' | 'postgres'
  dsn?: string
}

/** Joins a base URL and a path without doubling the slash. */
export function apiURL(baseURL: string, path: string): string {
  return `${baseURL.replace(/\/+$/, '')}${path}`
}

/** Waits until the server answers on the install status endpoint. */
export async function waitForServer(baseURL: string, timeout = 60000): Promise<void> {
  const deadline = Date.now() + timeout
  let lastError: unknown

  while (Date.now() < deadline) {
    try {
      const response = await fetch(apiURL(baseURL, '/api/install/status'))
      if (response.ok) {
        return
      }
      lastError = new Error(`status ${response.status}`)
    } catch (error) {
      lastError = error
    }
    await new Promise((resolve) => setTimeout(resolve, 250))
  }

  throw new Error(`server at ${baseURL} was not ready within ${timeout}ms: ${lastError}`)
}

/** Reads the installation status of a server. */
export async function installStatus(baseURL: string): Promise<InstallStatus> {
  const response = await fetch(apiURL(baseURL, '/api/install/status'))
  if (!response.ok) {
    throw new Error(`install status failed: ${response.status} ${await response.text()}`)
  }
  const payload = await response.json()
  return payload.result as InstallStatus
}

/**
 * Runs the wizard's "test connection" probe. Returns the HTTP status with the
 * decoded body instead of throwing: the failing cases are the interesting ones.
 */
export async function testDatabaseConnection(
  baseURL: string,
  choice: DatabaseChoice
): Promise<{ status: number; success: boolean; result: any; message: string }> {
  const response = await fetch(apiURL(baseURL, '/api/install/db/test'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(choice)
  })
  const text = await response.text()
  const payload = text ? JSON.parse(text) : {}
  return {
    status: response.status,
    success: !!payload.success,
    result: payload.result,
    message: payload.message
  }
}

/** Signs in over HTTP and returns the session token. */
export async function signIn(
  baseURL: string,
  email: string = ADMIN_EMAIL,
  password: string = ADMIN_PASSWORD
): Promise<string> {
  const response = await fetch(apiURL(baseURL, '/api/sign/in'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password })
  })
  if (response.status === 429) {
    throw new Error(
      'sign in was rate limited: sign-in, installation and payment share a limit of ten ' +
        'requests a minute per address, and the suite has already spent them all'
    )
  }
  if (!response.ok) {
    throw new Error(`sign in failed: ${response.status} ${await response.text()}`)
  }

  const setCookie = response.headers.get('set-cookie') ?? ''
  const match = setCookie.match(/token=([^;]+)/)
  if (!match) {
    throw new Error('sign in returned no token cookie')
  }
  return match[1]
}

/**
 * Where global setup leaves the session it established for the test workers.
 *
 * Sign-in, installation and payment share a limit of ten requests a minute per
 * address, and the whole suite calls from one address, so the session is
 * established once and handed on instead of being recreated per fixture.
 */
const SESSION_FILE = path.resolve(__dirname, '..', '.auth', 'admin-session.json')

interface StoredSession {
  baseURL: string
  token: string
}

/** Drops a trailing slash, so `http://host:1/` and `http://host:1` match. */
function normalizeBaseURL(baseURL: string): string {
  return baseURL.replace(/\/+$/, '')
}

/** Records a session for the test workers to pick up. */
export function saveSession(baseURL: string, token: string): void {
  const session: StoredSession = { baseURL: normalizeBaseURL(baseURL), token }
  fs.mkdirSync(path.dirname(SESSION_FILE), { recursive: true })
  fs.writeFileSync(SESSION_FILE, JSON.stringify(session))
}

/** The recorded session, when one was recorded for this server. */
function loadSession(baseURL: string): string | null {
  try {
    const session = JSON.parse(fs.readFileSync(SESSION_FILE, 'utf8')) as StoredSession
    return session.baseURL === normalizeBaseURL(baseURL) ? session.token : null
  } catch {
    return null
  }
}

const sessions = new Map<string, Promise<string>>()

/**
 * One admin session per server per process: the one global setup established,
 * or a fresh sign-in when the suite is run against a server it did not set up.
 */
export function sessionToken(baseURL: string): Promise<string> {
  let session = sessions.get(baseURL)
  if (!session) {
    const stored = loadSession(baseURL)
    session = (stored ? Promise.resolve(stored) : signIn(baseURL)).catch((error) => {
      sessions.delete(baseURL)
      throw error
    })
    sessions.set(baseURL, session)
  }
  return session
}

/**
 * Admin API bound to one session. Products created through it are deleted
 * again on dispose, so a run does not leave the storefront behind.
 */
export class AdminApi {
  private readonly createdProducts: string[] = []

  private constructor(
    private readonly baseURL: string,
    private readonly token: string
  ) {}

  static async connect(baseURL: string): Promise<AdminApi> {
    return new AdminApi(baseURL, await sessionToken(baseURL))
  }

  private async request<T>(path: string, method: string, body?: unknown): Promise<T> {
    const init: RequestInit = {
      method,
      headers: { 'Content-Type': 'application/json', Cookie: `token=${this.token}` }
    }
    if (body !== undefined) {
      init.body = JSON.stringify(body)
    }
    const response = await fetch(apiURL(this.baseURL, path), init)
    const text = await response.text()
    if (!response.ok) {
      throw new Error(`${method} ${path} failed: ${response.status} ${text}`)
    }
    const payload = text ? JSON.parse(text) : {}
    return (payload.result ?? payload) as T
  }

  /** Creates an active product. Without an amount it costs 0 — free. */
  async createProduct(input: ProductInput): Promise<Product> {
    const product = await this.request<Product>('/api/_/products', 'POST', {
      active: true,
      ...input
    })
    this.createdProducts.push(product.id)
    return product
  }

  async products(limit = 100): Promise<Product[]> {
    const page = await this.request<{ products: Product[] }>(`/api/_/products?limit=${limit}`, 'GET')
    return page.products ?? []
  }

  async getProduct(id: string): Promise<Product> {
    return this.request<Product>(`/api/_/products/${id}`, 'GET')
  }

  /**
   * Patches a product. The endpoint rewrites every column it owns, so the
   * current row is read first and the patch is merged into it.
   */
  async updateProduct(id: string, patch: Partial<ProductInput>): Promise<Product> {
    const current = await this.getProduct(id)
    return this.request<Product>(`/api/_/products/${id}`, 'PATCH', { ...current, ...patch })
  }

  /**
   * Activates or deactivates a product. The endpoint toggles the flag rather
   * than setting it, so the current state is read first — otherwise calling
   * this twice would leave the product the way it started.
   */
  async setActive(id: string, active: boolean): Promise<void> {
    const product = await this.getProduct(id)
    if (product.active === active) {
      return
    }
    await this.request(`/api/_/products/${id}/active`, 'PATCH', { active })
  }

  async carts(limit = 50): Promise<CartSummary[]> {
    const page = await this.request<{ carts: CartSummary[] }>(`/api/_/carts?limit=${limit}`, 'GET')
    return page.carts ?? []
  }

  /** Deletes what this session created. Best effort: it must not fail a test. */
  async dispose(): Promise<void> {
    for (const id of [...this.createdProducts].reverse()) {
      try {
        await this.request(`/api/_/products/${id}`, 'DELETE')
      } catch {
        // A product a cart already refers to cannot be deleted; that is fine.
      }
    }
    this.createdProducts.length = 0
  }
}
