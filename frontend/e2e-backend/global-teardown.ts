import { killTree } from './harness'

export default async function globalTeardown(): Promise<void> {
  const vitePid = process.env.E2E_VITE_PID
  const backendPid = process.env.E2E_BACKEND_PID
  if (vitePid) killTree(Number(vitePid))
  if (backendPid) killTree(Number(backendPid))
  const workDir = process.env.E2E_WORK_DIR
  if (workDir) console.log(`[e2e-backend] stopped stack; logs kept in ${workDir}`)
}
