// Servers send markdown for hover and signature text, and markdown may carry HTML. Only plain
// text formatting and http(s) links get through; every other tag becomes its text.
const ALLOWED = new Set(['P', 'BR', 'CODE', 'PRE', 'EM', 'STRONG', 'B', 'I', 'DEL', 'UL', 'OL', 'LI', 'SPAN', 'DIV', 'A', 'BLOCKQUOTE', 'HR', 'H1', 'H2', 'H3', 'H4', 'H5', 'H6', 'TABLE', 'THEAD', 'TBODY', 'TR', 'TH', 'TD'])

function clean(parent: Element) {
  for (const el of [...parent.children]) {
    if (!ALLOWED.has(el.tagName)) {
      el.replaceWith(document.createTextNode(el.textContent ?? ''))
      continue
    }
    for (const attr of [...el.attributes]) {
      const keep = attr.name === 'class' || (el.tagName === 'A' && attr.name === 'href' && /^https?:\/\//i.test(attr.value))
      if (!keep) el.removeAttribute(attr.name)
    }
    if (el.tagName === 'A') {
      el.setAttribute('target', '_blank')
      el.setAttribute('rel', 'noopener noreferrer')
    }
    clean(el)
  }
}

export function sanitizeHTML(html: string): string {
  const doc = new DOMParser().parseFromString(html, 'text/html')
  clean(doc.body)
  return doc.body.innerHTML
}
