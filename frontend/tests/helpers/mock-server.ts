import { createServer, type Server } from 'node:http'
import { createMockApiMiddleware } from '../../mock/game-mock.ts'

export interface MockApiServer {
  origin: string
  apiBase: string
  close: () => Promise<void>
}

export async function startMockApiServer(): Promise<MockApiServer> {
  const middleware = createMockApiMiddleware()
  const server: Server = createServer((req, res) => {
    middleware(req, res, () => {
      res.statusCode = 404
      res.setHeader('Content-Type', 'application/json')
      res.end(JSON.stringify({ error: { code: 'NOT_FOUND', message: 'Not found' } }))
    })
  })

  await new Promise<void>((resolve, reject) => {
    server.once('error', reject)
    server.listen(0, '127.0.0.1', () => resolve())
  })

  const address = server.address()
  if (address === null || typeof address === 'string') {
    throw new Error('Mock API server failed to bind a TCP port')
  }

  const origin = `http://127.0.0.1:${address.port}`
  return {
    origin,
    apiBase: `${origin}/api/v1`,
    close: () =>
      new Promise<void>((resolve, reject) => {
        server.close((err) => (err ? reject(err) : resolve()))
      }),
  }
}
