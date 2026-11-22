#!/usr/bin/env node
// Copies the official MCP registry (latest version of every server) into
// internal/mcp/registry.json.gz (gzip: the plain JSON is over 15 MB), keeping only the fields
// the catalog uses. No packages.
// Usage: node scripts/mcp-registry.mjs [output-file]
import { writeFileSync } from 'node:fs'
import { gzipSync } from 'node:zlib'
import { fileURLToPath } from 'node:url'

const base = 'https://registry.modelcontextprotocol.io/v0/servers'
const out = process.argv[2] ?? fileURLToPath(new URL('../internal/mcp/registry.json.gz', import.meta.url))
const headers = { 'User-Agent': 'mooring-registry-script', Accept: 'application/json' }
const maxPages = 400

const cut = (s, n) => (typeof s === 'string' ? [...s].slice(0, n).join('') : undefined)
const vars = (list) =>
  (Array.isArray(list) ? list : [])
    .filter((v) => v && typeof v.name === 'string')
    .map((v) => {
      const o = { name: v.name }
      if (v.description) o.description = cut(v.description, 100)
      if (v.isRequired) o.isRequired = true
      if (v.isSecret) o.isSecret = true
      return o
    })

function slim(s) {
  const out = { name: s.name, title: cut(s.title, 60), description: cut(s.description, 120), version: s.version }
  if (typeof s.websiteUrl === 'string') out.websiteUrl = s.websiteUrl
  if (s.repository && typeof s.repository.url === 'string') out.repository = { url: s.repository.url }
  const packages = (s.packages ?? [])
    .filter((p) => ['npm', 'pypi', 'oci'].includes(p.registryType) && (p.transport?.type ?? 'stdio') === 'stdio')
    .map((p) => {
      const o = { registryType: p.registryType, identifier: p.identifier, version: p.version }
      const env = vars(p.environmentVariables)
      if (env.length) o.environmentVariables = env
      return o
    })
  const remotes = (s.remotes ?? [])
    .filter((r) => ['streamable-http', 'sse'].includes(r.type) && typeof r.url === 'string')
    .map((r) => {
      const o = { type: r.type, url: r.url }
      const h = vars(r.headers)
      if (h.length) o.headers = h
      return o
    })
  if (packages.length) out.packages = packages
  if (remotes.length) out.remotes = remotes
  return packages.length || remotes.length ? out : null
}

const seen = new Map()
let cursor = ''
for (let page = 0; page < maxPages; page++) {
  const url = `${base}?limit=100&version=latest${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ''}`
  const res = await fetch(url, { headers })
  if (!res.ok) throw new Error(`registry answered ${res.status} for page ${page}`)
  const body = await res.json()
  for (const item of body.servers ?? []) {
    const status = item._meta?.['io.modelcontextprotocol.registry/official']?.status
    if (status && status !== 'active') continue
    const s = slim(item.server ?? {})
    if (s && s.name && !seen.has(s.name)) seen.set(s.name, { server: s })
  }
  cursor = body.metadata?.nextCursor ?? ''
  if (!cursor) break
}
const servers = [...seen.values()].sort((a, b) => (a.server.name < b.server.name ? -1 : 1))
const json = JSON.stringify({ date: new Date().toISOString().slice(0, 10), servers }) + '\n'
writeFileSync(out, gzipSync(json, { level: 9 }))
console.log(`${servers.length} servers written to ${out}`)
