import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'
import { createMockApiMiddleware } from './mock/game-mock.ts'

export default defineConfig(({ mode }) => {
  const useMock = mode === 'mock'
  return {
    plugins: [
      react(),
      {
        name: 'delivery-office-mock-api',
        configureServer(server) {
          if (useMock) {
            server.middlewares.use(createMockApiMiddleware())
          }
        },
      },
    ],
    server: {
      proxy: useMock
        ? undefined
        : {
            '/api': {
              target: process.env.VITE_BACKEND_URL ?? 'http://127.0.0.1:8080',
              changeOrigin: true,
            },
          },
    },
  }
})
