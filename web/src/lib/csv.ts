// A small reader for comma and tab separated text: quoted fields, doubled quotes, line breaks
// inside quotes, and CRLF. It stops after `maxRows` rows so a huge file is not read in full.

export interface Csv {
  rows: string[][]
  /** True when the text held more rows than `maxRows`. */
  more: boolean
}

/** The separator a file name implies: a tab for .tsv, a comma otherwise. */
export const delimiterFor = (path: string): string => (/\.tsv$/i.test(path) ? '\t' : ',')

export function parseCsv(text: string, delimiter = ',', maxRows = Infinity): Csv {
  const src = text.charCodeAt(0) === 0xfeff ? text.slice(1) : text
  const rows: string[][] = []
  let row: string[] = []
  let field = ''
  let quoted = false
  let started = false

  // A row ends here. A blank line is not a row.
  const endRow = (): boolean => {
    if (started || row.length > 0) {
      row.push(field)
      rows.push(row)
    }
    row = []
    field = ''
    started = false
    return rows.length > maxRows
  }

  for (let i = 0; i < src.length; i++) {
    const c = src[i]
    if (quoted) {
      if (c !== '"') field += c
      else if (src[i + 1] === '"') (field += '"'), i++
      else quoted = false
    } else if (c === '"' && field === '') {
      quoted = true
      started = true
    } else if (c === delimiter) {
      row.push(field)
      field = ''
      started = true
    } else if (c === '\n' || c === '\r') {
      if (c === '\r' && src[i + 1] === '\n') i++
      if (endRow()) return { rows: rows.slice(0, maxRows), more: true }
    } else {
      field += c
      started = true
    }
  }
  const over = endRow()
  return { rows: rows.slice(0, maxRows), more: over }
}
