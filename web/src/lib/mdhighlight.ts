// Code highlighting for chat code blocks. It reuses the language packages the editor ships, one
// dynamic import per language, and the parse is the editor's own. Anything that fails stays plain text.
import { LanguageSupport, type Language } from '@codemirror/language'
import { highlightTree, tagHighlighter, tags as t } from '@lezer/highlight'
import { languageFor, type LangKey } from './langs'
import { loadLanguage } from './cmlang'

export interface Run {
  text: string
  cls: string
}

const ALIAS: Record<string, string> = {
  javascript: 'js', typescript: 'ts', python: 'py', golang: 'go', bash: 'sh', shell: 'sh', zsh: 'sh', console: 'sh', yml: 'yaml',
  rust: 'rs', ruby: 'rb', markdown: 'md', 'c++': 'cpp', 'c#': '', jsonc: 'json', makefile: 'sh', toml: 'toml', dockerfile: 'dockerfile',
}

export function langKeyOf(info: string): LangKey | null {
  const name = info.toLowerCase()
  if (!name) return null
  if (name === 'dockerfile') return 'dockerfile'
  return languageFor('x.' + (ALIAS[name] ?? name))
}

const highlighter = tagHighlighter([
  { tag: [t.keyword, t.operatorKeyword, t.modifier, t.controlKeyword], class: 'tk-kw' },
  { tag: [t.string, t.special(t.string), t.regexp], class: 'tk-str' },
  { tag: [t.number, t.bool, t.null, t.atom], class: 'tk-num' },
  { tag: [t.comment, t.lineComment, t.blockComment, t.docComment], class: 'tk-cm' },
  { tag: [t.typeName, t.className, t.namespace, t.tagName], class: 'tk-ty' },
  { tag: [t.function(t.variableName), t.function(t.propertyName), t.definition(t.variableName)], class: 'tk-fn' },
  { tag: [t.propertyName, t.attributeName, t.labelName], class: 'tk-pr' },
])

const CAP = 20_000
const cache = new Map<string, Run[] | null>()

/** The code as runs of text with a colour class, or null for plain text (unknown language, too long, load failed). */
export async function highlight(code: string, info: string): Promise<Run[] | null> {
  const key = langKeyOf(info)
  if (!key || code.length > CAP) return null
  const ck = key + '\0' + code
  if (cache.has(ck)) return cache.get(ck)!
  let runs: Run[] | null = null
  try {
    const ext = await loadLanguage(key)
    const lang = ext instanceof LanguageSupport ? ext.language : (ext as Language | never[])
    if (lang && !Array.isArray(lang) && 'parser' in lang) {
      const tree = lang.parser.parse(code)
      const out: Run[] = []
      let at = 0
      highlightTree(tree, highlighter, (from, to, cls) => {
        if (from > at) out.push({ text: code.slice(at, from), cls: '' })
        out.push({ text: code.slice(from, to), cls })
        at = to
      })
      if (at < code.length) out.push({ text: code.slice(at), cls: '' })
      runs = out
    }
  } catch {
    runs = null
  }
  if (cache.size > 200) cache.delete(cache.keys().next().value!)
  cache.set(ck, runs)
  return runs
}
