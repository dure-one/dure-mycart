import { spawn, ChildProcess } from 'child_process'
import { createHash } from 'crypto'
import * as fs from 'fs'
import { createServer } from 'net'
import * as os from 'os'
import * as path from 'path'

import { waitForServer } from './api'

/**
 * A myCart process of its own, with a store of its own.
 *
 * The install wizard only exists on a server that has not been installed yet,
 * and the rest of the suite runs against an installed store on the shared
 * server. This starts a second process for those tests: built from the current
 * working tree, listening on a port of its own, and running in a temporary
 * directory because `lc_base/data.db` and `lc_base/config.json` are resolved
 * relative to the working directory.
 */

const REPO_ROOT = path.resolve(__dirname, '..', '..')

export interface IsolatedServer {
  /** e.g. http://127.0.0.1:53211 */
  baseURL: string
  port: number
  /** Working directory of the process: holds its database and config. */
  dataDir: string
  stop: () => Promise<void>
}

let binary: Promise<string> | undefined

/** Builds the server once per run, outside any data directory. */
function serverBinary(): Promise<string> {
  if (!binary) {
    // Keyed by repository so parallel checkouts do not overwrite each other's.
    const tag = createHash('sha1').update(REPO_ROOT).digest('hex').slice(0, 8)
    const target = path.join(os.tmpdir(), `mycart-e2e-${tag}`, 'mycart')

    binary = new Promise<string>((resolve, reject) => {
      fs.mkdirSync(path.dirname(target), { recursive: true })
      const build = spawn('go', ['build', '-o', target, './cmd'], { cwd: REPO_ROOT })
      let output = ''
      build.stdout.on('data', (chunk) => (output += chunk))
      build.stderr.on('data', (chunk) => (output += chunk))
      build.on('error', reject)
      build.on('close', (code) => {
        if (code === 0) {
          resolve(target)
        } else {
          reject(new Error(`go build ./cmd failed with ${code}:\n${output}`))
        }
      })
    })
  }
  return binary
}

/** Asks the OS for a port nothing is listening on. */
function freePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const probe = createServer()
    probe.on('error', reject)
    probe.listen(0, '127.0.0.1', () => {
      const address = probe.address()
      const port = typeof address === 'object' && address ? address.port : 0
      probe.close(() => resolve(port))
    })
  })
}

function stopProcess(child: ChildProcess): Promise<void> {
  return new Promise((resolve) => {
    if (child.exitCode !== null || child.signalCode !== null) {
      resolve()
      return
    }
    child.once('exit', () => resolve())
    child.kill('SIGTERM')
    setTimeout(() => {
      child.kill('SIGKILL')
      resolve()
    }, 3000).unref()
  })
}

/**
 * Starts an uninstalled server. Environment variables passed here are how a
 * test pins the database (`MYCART_DB_DSN`), which is what turns the wizard's
 * engine selection into read-only text.
 */
export async function startIsolatedServer(env: NodeJS.ProcessEnv = {}): Promise<IsolatedServer> {
  const executable = await serverBinary()
  const port = await freePort()
  const dataDir = fs.mkdtempSync(path.join(os.tmpdir(), 'mycart-e2e-data-'))

  const child = spawn(executable, ['serve', '--http', `127.0.0.1:${port}`], {
    cwd: dataDir,
    env: { ...process.env, ...env },
    stdio: 'pipe'
  })

  let log = ''
  let ready = false
  const collect = (chunk: Buffer) => (log += chunk.toString())
  child.stdout?.on('data', collect)
  child.stderr?.on('data', collect)

  const startup = new Promise<never>((_, reject) => {
    child.on('error', reject)
    child.on('exit', (code, signal) => {
      if (!ready) {
        reject(new Error(`server exited before it was ready (${signal ?? code}):\n${log}`))
      }
    })
  })

  const baseURL = `http://127.0.0.1:${port}`
  try {
    await Promise.race([waitForServer(baseURL, 30000), startup])
  } catch (error) {
    await stopProcess(child)
    throw error
  }
  ready = true

  return {
    baseURL,
    port,
    dataDir,
    stop: async () => {
      await stopProcess(child)
      fs.rmSync(dataDir, { recursive: true, force: true })
    }
  }
}
