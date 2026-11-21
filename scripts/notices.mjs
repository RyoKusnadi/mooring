#!/usr/bin/env node
// Writes THIRD_PARTY_NOTICES.txt: the licence of every Go module compiled into cmd/mooring and of
// every web package bundled into the app. Run it from anywhere after `go mod download` and
// `pnpm install`; commit the result. It reads go.mod through `go list`, never go.sum or the lockfile.
// Usage: node scripts/notices.mjs [--check]   (--check fails if the file would change)
import { execFileSync } from 'node:child_process'
import { existsSync, readFileSync, readdirSync, realpathSync, writeFileSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const out = join(root, 'THIRD_PARTY_NOTICES.txt')
const licenceName = /^(licen[cs]e|copying|notice|unlicense)([-._].*)?$/i
// Packages the app bundles that web/package.json lists as devDependencies.
const bundledDev = ['svelte']

const missing = []

function licenceFiles(dir) {
  return readdirSync(dir)
    .filter((f) => licenceName.test(f) && !f.endsWith('.js'))
    .sort()
    .map((f) => ({ file: f, text: readFileSync(join(dir, f), 'utf8').trim() }))
    .filter((l) => l.text)
}

function entry(title, source, licence, dir) {
  const files = licenceFiles(dir)
  if (files.length === 0) missing.push(`${title} (declared licence: ${licence || 'none'})`)
  const body = files.length
    ? files.map((l) => `--- ${l.file} ---\n${l.text}`).join('\n\n')
    : `No licence file ships with this package. Declared licence: ${licence || 'none declared'}.`
  return `${title}\nSource: ${source}\nLicence: ${licence || 'see below'}\n\n${body}`
}

function goEntries() {
  const raw = execFileSync(
    'go',
    ['list', '-deps', '-f', '{{with .Module}}{{.Path}}\t{{.Version}}\t{{.Dir}}\t{{.Main}}{{end}}', './cmd/mooring'],
    { cwd: root, encoding: 'utf8', env: { ...process.env, CGO_ENABLED: '0' } },
  )
  const mods = new Map()
  for (const line of raw.split('\n')) {
    if (!line) continue
    const [path, version, dir, main] = line.split('\t')
    if (main === 'true' || mods.has(path)) continue
    mods.set(path, { version, dir })
  }
  const items = [...mods.keys()].sort().map((path) => {
    const { version, dir } = mods.get(path)
    return entry(`${path} ${version}`, `https://${path}`, '', dir)
  })
  const goroot = execFileSync('go', ['env', 'GOROOT'], { encoding: 'utf8' }).trim()
  // Some packaged Go toolchains (Homebrew) keep LICENSE one folder above GOROOT.
  const licenceDir = existsSync(join(goroot, 'LICENSE')) ? goroot : dirname(goroot)
  items.unshift(entry('Go standard library', 'https://go.dev', 'BSD-3-Clause', licenceDir))
  return items
}

// Finds name in the node_modules folders above dir, the way Node does.
function findPackage(name, dir) {
  for (let d = dir; ; d = dirname(d)) {
    const p = join(d, 'node_modules', name)
    if (existsSync(join(p, 'package.json'))) return realpathSync(p)
    if (dirname(d) === d) return null
  }
}

function webEntries() {
  const web = join(root, 'web')
  const pkg = JSON.parse(readFileSync(join(web, 'package.json'), 'utf8'))
  const seen = new Map()
  const queue = [...Object.keys(pkg.dependencies ?? {}), ...bundledDev].map((n) => [n, web])
  while (queue.length) {
    const [name, from] = queue.shift()
    const dir = findPackage(name, from)
    if (!dir) {
      if (!seen.has(name)) missing.push(`${name} (not installed, run pnpm install)`)
      continue
    }
    if (seen.has(dir)) continue
    const meta = JSON.parse(readFileSync(join(dir, 'package.json'), 'utf8'))
    seen.set(dir, { name: meta.name ?? name, meta, dir })
    for (const dep of Object.keys(meta.dependencies ?? {})) queue.push([dep, dir])
  }
  return [...seen.values()]
    .sort((a, b) => (a.name + a.meta.version).localeCompare(b.name + b.meta.version))
    .map(({ name, meta, dir }) => {
      const licence = typeof meta.license === 'string' ? meta.license : meta.license?.type
      return entry(`${name} ${meta.version}`, `https://www.npmjs.com/package/${name}`, licence, dir)
    })
}

const rule = '\n\n' + '='.repeat(72) + '\n\n'
const text = [
  'Third-party notices for Mooring',
  '',
  'Mooring itself is MIT licensed (see LICENSE). This file lists the software that is compiled or',
  'bundled into the Mooring binary, with its licence text. It is made by scripts/notices.mjs.',
  '',
  'The file internal/mcp/registry.json.gz is a snapshot of the public MCP server registry',
  '(https://registry.modelcontextprotocol.io), made by scripts/mcp-registry.mjs.',
  '',
  '#### Go modules',
  rule + goEntries().join(rule),
  '',
  '#### Web packages bundled into the app',
  rule + webEntries().join(rule),
].join('\n') + '\n'

if (process.argv.includes('--check')) {
  const current = existsSync(out) ? readFileSync(out, 'utf8') : ''
  if (current !== text) {
    console.error('notices: THIRD_PARTY_NOTICES.txt is out of date. Run node scripts/notices.mjs.')
    process.exit(1)
  }
} else {
  writeFileSync(out, text)
  console.log(`notices: wrote ${out}`)
}
if (missing.length) {
  console.error(`notices: no licence file for ${missing.length} package(s):\n  ${missing.join('\n  ')}`)
}
