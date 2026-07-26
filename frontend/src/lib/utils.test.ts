import { describe, expect, it } from 'vitest'
import { groupBy } from './utils'

describe('groupBy', () => {
  it('buckets items by key, preserving item order within a bucket', () => {
    const items = [
      { id: 1, k: 'a' },
      { id: 2, k: 'b' },
      { id: 3, k: 'a' },
    ]
    expect(groupBy(items, (i) => i.k)).toEqual([
      ['a', [items[0], items[2]]],
      ['b', [items[1]]],
    ])
  })

  it('orders groups by first-seen key, not alphabetically', () => {
    const items = [{ k: 'z' }, { k: 'a' }, { k: 'z' }]
    const result = groupBy(items, (i) => i.k)
    expect(result.map(([key]) => key)).toEqual(['z', 'a'])
  })

  it('returns an empty array for an empty input', () => {
    expect(groupBy<{ k: string }>([], (i) => i.k)).toEqual([])
  })
})
