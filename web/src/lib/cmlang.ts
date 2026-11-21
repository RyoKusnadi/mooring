// Language modes for the editor, one dynamic import each, so a file pulls in only its own parser.
import type { Extension } from '@codemirror/state'
import { StreamLanguage, type StreamParser } from '@codemirror/language'
import type { LangKey } from './langs'

const legacy = async (parser: Promise<StreamParser<unknown>>): Promise<Extension> => StreamLanguage.define(await parser)

const LOADERS: Record<LangKey, () => Promise<Extension>> = {
  js: async () => (await import('@codemirror/lang-javascript')).javascript(),
  jsx: async () => (await import('@codemirror/lang-javascript')).javascript({ jsx: true }),
  ts: async () => (await import('@codemirror/lang-javascript')).javascript({ typescript: true }),
  tsx: async () => (await import('@codemirror/lang-javascript')).javascript({ typescript: true, jsx: true }),
  json: async () => (await import('@codemirror/lang-json')).json(),
  css: async () => (await import('@codemirror/lang-css')).css(),
  html: async () => (await import('@codemirror/lang-html')).html(),
  svelte: async () => (await import('@codemirror/lang-html')).html(),
  markdown: async () => (await import('@codemirror/lang-markdown')).markdown(),
  python: async () => (await import('@codemirror/lang-python')).python(),
  go: async () => (await import('@codemirror/lang-go')).go(),
  rust: async () => (await import('@codemirror/lang-rust')).rust(),
  yaml: async () => (await import('@codemirror/lang-yaml')).yaml(),
  sql: async () => (await import('@codemirror/lang-sql')).sql(),
  cpp: async () => (await import('@codemirror/lang-cpp')).cpp(),
  c: async () => (await import('@codemirror/lang-cpp')).cpp(),
  java: async () => (await import('@codemirror/lang-java')).java(),
  php: async () => (await import('@codemirror/lang-php')).php(),
  xml: async () => (await import('@codemirror/lang-xml')).xml(),
  shell: () => legacy(import('@codemirror/legacy-modes/mode/shell').then((m) => m.shell as StreamParser<unknown>)),
  toml: () => legacy(import('@codemirror/legacy-modes/mode/toml').then((m) => m.toml as StreamParser<unknown>)),
  dockerfile: () => legacy(import('@codemirror/legacy-modes/mode/dockerfile').then((m) => m.dockerFile as StreamParser<unknown>)),
  ruby: () => legacy(import('@codemirror/legacy-modes/mode/ruby').then((m) => m.ruby as StreamParser<unknown>)),
  swift: () => legacy(import('@codemirror/legacy-modes/mode/swift').then((m) => m.swift as StreamParser<unknown>)),
  lua: () => legacy(import('@codemirror/legacy-modes/mode/lua').then((m) => m.lua as StreamParser<unknown>)),
}

/** The mode for a language, or an empty extension if it cannot load (the file then shows as plain text). */
export async function loadLanguage(key: LangKey | null): Promise<Extension> {
  if (!key) return []
  try {
    return await LOADERS[key]()
  } catch {
    return []
  }
}
