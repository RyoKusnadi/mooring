import { expect, test } from 'vitest'
import { termFontSize } from './termfont'

test('the terminal font follows the text size setting', () => {
  expect(termFontSize('small')).toBe(12)
  expect(termFontSize('default')).toBe(13)
  expect(termFontSize('large')).toBe(15)
  expect(termFontSize(undefined)).toBe(13)
})
