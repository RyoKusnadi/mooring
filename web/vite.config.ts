/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'
import { createRequire } from 'node:module'

// pnpm keeps @codemirror/lint next to the language client that needs it, not in our own node_modules.
// The gutter marks come from it, so it is found the way the client finds it.
const fromClient = createRequire(createRequire(import.meta.url).resolve('@codemirror/lsp-client'))

export default defineConfig({
  plugins: [svelte()],
  resolve: { alias: { '@codemirror/lint': fromClient.resolve('@codemirror/lint').replace(/index\.cjs$/, 'index.js') } },
  build: {
    // The Go binary embeds this folder (internal/webui).
    outDir: '../internal/webui/dist',
    emptyOutDir: true,
    target: 'es2022',
  },
  server: {
    // `make run` serves the API on this port; `pnpm dev` adds hot reload on top.
    proxy: { '/api': 'http://127.0.0.1:7340' },
  },
  test: {
    include: ['src/**/*.test.ts'],
    environment: 'node',
  },
})
