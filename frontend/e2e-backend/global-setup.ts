import { spawn, spawnSync, type ChildProcess } from 'node:child_process'
import { mkdtempSync, openSync } from 'node:fs'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { BACKEND_PORT, FRONTEND_PORT, killTree, portResponds, waitForReady } from './harness'

const thisDir = path.dirname(fileURLToPath(import.meta.url))
const frontendDir = path.resolve(thisDir, '..')
const backendDir = path.resolve(frontendDir, '..', 'backend')

function childEnv(extra: Record<string, string>): NodeJS.ProcessEnv {
  return {
    ...process.env,
    ...extra,
    AGENT_OWNER: 'opencode',
    AGENT_TASK: 'e2e-backend',
  }
}

export default async function globalSetup(): Promise<void> {
  const backendUrl = `http://127.0.0.1:${BACKEND_PORT}/api/v1/game`
  const frontendUrl = `http://localhost:${FRONTEND_PORT}/`

  if (await portResponds(backendUrl)) {
    throw new Error(
      `Port ${BACKEND_PORT} already serves traffic — a previous e2e backend may still be running. Stop it and rerun.`,
    )
  }
  if (await portResponds(frontendUrl)) {
    throw new Error(
      `Port ${FRONTEND_PORT} already serves traffic — a previous Vite server may still be running. Stop it and rerun.`,
    )
  }

  const workDir = mkdtempSync(path.join(tmpdir(), 'postman-e2e-'))
  const bin = path.join(workDir, process.platform === 'win32' ? 'postman.exe' : 'postman')

  const build = spawnSync('go', ['build', '-o', bin, './cmd/postman'], {
    cwd: backendDir,
    env: childEnv({}),
    stdio: 'inherit',
  })
  if (build.status !== 0) {
    throw new Error(`go build failed for the backend (exit ${build.status})`)
  }

  const logFd = openSync(path.join(workDir, 'servers.log'), 'a')
  let backend: ChildProcess | undefined
  let vite: ChildProcess | undefined
  try {
    backend = spawn(bin, [], {
      cwd: backendDir,
      env: childEnv({
        BACKEND_ADDR: `127.0.0.1:${BACKEND_PORT}`,
        POSTMAN_DB_PATH: path.join(workDir, 'postman.db'),
        POSTMAN_LOG_FILE: path.join(workDir, 'postman.log'),
      }),
      stdio: ['ignore', logFd, logFd],
    })

    const viteBin = path.join(frontendDir, 'node_modules', 'vite', 'bin', 'vite.js')
    vite = spawn(process.execPath, [viteBin, '--port', String(FRONTEND_PORT), '--strictPort'], {
      cwd: frontendDir,
      env: childEnv({ VITE_BACKEND_URL: `http://127.0.0.1:${BACKEND_PORT}` }),
      stdio: ['ignore', logFd, logFd],
    })

    if (backend.pid !== undefined) process.env.E2E_BACKEND_PID = String(backend.pid)
    if (vite.pid !== undefined) process.env.E2E_VITE_PID = String(vite.pid)
    process.env.E2E_WORK_DIR = workDir

    await waitForReady(backendUrl, 'Go backend')
    await waitForReady(frontendUrl, 'Vite dev server')
    console.log(`[e2e-backend] stack ready (logs: ${workDir})`)
  } catch (err) {
    if (backend?.pid !== undefined) killTree(backend.pid)
    if (vite?.pid !== undefined) killTree(vite.pid)
    console.error(`[e2e-backend] setup failed; server logs are in ${workDir}`)
    throw err
  }
}
