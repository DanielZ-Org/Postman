import { spawnSync } from 'node:child_process'

export const BACKEND_PORT = 8098
export const FRONTEND_PORT = 5176
export const READY_TIMEOUT_MS = 60_000

export async function portResponds(url: string): Promise<boolean> {
  try {
    const res = await fetch(url, { signal: AbortSignal.timeout(1_000) })
    return res.status > 0
  } catch {
    return false
  }
}

export async function waitForReady(url: string, label: string): Promise<void> {
  const deadline = Date.now() + READY_TIMEOUT_MS
  while (Date.now() < deadline) {
    if (await portResponds(url)) return
    await new Promise((resolve) => setTimeout(resolve, 250))
  }
  throw new Error(`${label} did not become ready within ${READY_TIMEOUT_MS} ms: ${url}`)
}

export function killTree(pid: number): void {
  if (!Number.isInteger(pid) || pid <= 0) return
  try {
    if (process.platform === 'win32') {
      spawnSync('taskkill', ['/pid', String(pid), '/T', '/F'], { stdio: 'ignore' })
    } else {
      process.kill(pid, 'SIGTERM')
    }
  } catch (err) {
    console.warn(`[e2e-backend] failed to stop pid ${pid}:`, err)
  }
}
