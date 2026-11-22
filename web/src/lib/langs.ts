// Which language mode a file gets, by name. No editor code here, so it stays in the app shell.
export type LangKey =
  | 'js' | 'jsx' | 'ts' | 'tsx' | 'json' | 'css' | 'html' | 'markdown' | 'python' | 'go' | 'rust' | 'yaml'
  | 'sql' | 'cpp' | 'java' | 'php' | 'xml' | 'svelte' | 'shell' | 'toml' | 'dockerfile' | 'ruby' | 'swift' | 'lua' | 'c'

const BY_EXT: Record<string, LangKey> = {
  js: 'js', mjs: 'js', cjs: 'js', jsx: 'jsx', ts: 'ts', mts: 'ts', cts: 'ts', tsx: 'tsx',
  json: 'json', jsonc: 'json', webmanifest: 'json', css: 'css', scss: 'css', html: 'html', htm: 'html', vue: 'html', svelte: 'svelte',
  md: 'markdown', markdown: 'markdown', py: 'python', pyi: 'python', go: 'go', rs: 'rust', yaml: 'yaml', yml: 'yaml',
  sql: 'sql', c: 'c', h: 'cpp', cc: 'cpp', cpp: 'cpp', cxx: 'cpp', hpp: 'cpp', java: 'java', php: 'php',
  xml: 'xml', svg: 'xml', plist: 'xml', sh: 'shell', bash: 'shell', zsh: 'shell', toml: 'toml', rb: 'ruby', swift: 'swift', lua: 'lua',
}
const BY_NAME: Record<string, LangKey> = {
  dockerfile: 'dockerfile', makefile: 'shell', '.bashrc': 'shell', '.zshrc': 'shell', '.profile': 'shell',
  gemfile: 'ruby', rakefile: 'ruby', 'go.mod': 'go', '.gitignore': 'shell', '.env': 'shell',
}

/** The language of a file, or null for plain text. */
export function languageFor(path: string): LangKey | null {
  const name = path.slice(path.lastIndexOf('/') + 1).toLowerCase()
  if (BY_NAME[name]) return BY_NAME[name]
  const dot = name.lastIndexOf('.')
  if (dot < 0) return null
  return BY_EXT[name.slice(dot + 1)] ?? null
}
