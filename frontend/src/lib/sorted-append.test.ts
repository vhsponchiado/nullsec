import { describe, expect, it } from 'vitest'
import { insertSorted } from './sorted-append'

const numAsc = (a: number, b: number) => a - b

describe('insertSorted', () => {
  it('inserts into an empty array', () => {
    expect(insertSorted([], 5, numAsc)).toEqual([5])
  })

  it('inserts at the front', () => {
    expect(insertSorted([3, 5, 8], 1, numAsc)).toEqual([1, 3, 5, 8])
  })

  it('inserts in the middle', () => {
    expect(insertSorted([1, 3, 8], 5, numAsc)).toEqual([1, 3, 5, 8])
  })

  it('inserts at the end', () => {
    expect(insertSorted([1, 3, 5], 8, numAsc)).toEqual([1, 3, 5, 8])
  })

  it('inserts a duplicate value without disturbing relative order of ties', () => {
    expect(insertSorted([1, 3, 3, 5], 3, numAsc)).toEqual([1, 3, 3, 3, 5])
  })

  it('does not mutate the input array', () => {
    const original = [1, 3, 5]
    const result = insertSorted(original, 4, numAsc)
    expect(original).toEqual([1, 3, 5])
    expect(result).toEqual([1, 3, 4, 5])
  })

  it('works with a descending comparator', () => {
    const desc = (a: number, b: number) => b - a
    expect(insertSorted([8, 5, 3], 4, desc)).toEqual([8, 5, 4, 3])
  })

  it('works with object items and a field comparator', () => {
    const items = [{ n: 1 }, { n: 3 }, { n: 5 }]
    const byN = (a: { n: number }, b: { n: number }) => a.n - b.n
    expect(insertSorted(items, { n: 4 }, byN)).toEqual([{ n: 1 }, { n: 3 }, { n: 4 }, { n: 5 }])
  })

  it('building up a list one insert at a time matches sorting it all at once', () => {
    const input = [9, 2, 7, 1, 5, 3, 8, 4, 6, 0]
    let built: number[] = []
    for (const n of input) built = insertSorted(built, n, numAsc)
    expect(built).toEqual([...input].sort(numAsc))
  })
})
