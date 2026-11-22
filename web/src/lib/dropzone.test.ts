import { expect, it } from 'vitest'
import { insertIndex, zoneAt } from './dropzone'

const r = { left: 0, top: 0, width: 400, height: 200 }

it('maps the pointer to an edge or the middle', () => {
  expect(zoneAt(r, 10, 100)).toBe('left')
  expect(zoneAt(r, 390, 100)).toBe('right')
  expect(zoneAt(r, 200, 10)).toBe('top')
  expect(zoneAt(r, 200, 190)).toBe('bottom')
  expect(zoneAt(r, 200, 100)).toBe('center')
})

it('finds the tab slot under the pointer', () => {
  const tabs = [{ left: 0, width: 100 }, { left: 100, width: 100 }]
  expect(insertIndex(tabs, 20)).toBe(0)
  expect(insertIndex(tabs, 120)).toBe(1)
  expect(insertIndex(tabs, 180)).toBe(2)
  expect(insertIndex([], 5)).toBe(0)
})
