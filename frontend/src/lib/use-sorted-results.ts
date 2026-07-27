import { useEffect, useRef, useState } from 'react'
import { insertSorted } from './sorted-append'

/**
 * Keeps a live-streaming list (scan/attack results arriving one at a time
 * over the WebSocket) sorted by `compare`, without re-sorting the whole
 * array on every single new arrival: that's what the naive
 * `[...results].sort(...)` in a `useMemo` keyed on `results` does, and it
 * turns an N-result run into O(N² log N) of total sorting work by the time
 * it finishes, visibly stalling the UI as N grows into the thousands.
 *
 * Instead, each newly-appended item is inserted directly at its correct
 * position via binary search. A full sort only happens when `compare`
 * itself changes (the user picked a different column/direction; pass a
 * `useCallback`'d comparator so this is detected correctly), or when
 * `items` wasn't a pure append since the last render (some callers derive
 * `items` via a filter, e.g. Subdomain Finder's "show unresolved" toggle,
 * which can insert/remove items anywhere, not just at the tail; a plain
 * length check wouldn't catch that, so this checks that every
 * previously-seen item is still at the same index).
 */
export function useSortedResults<T>(items: T[], compare: (a: T, b: T) => number): T[] {
  const [sorted, setSorted] = useState<T[]>(() => [...items].sort(compare))
  const sortedRef = useRef(sorted)
  const prevItemsRef = useRef(items)
  const compareRef = useRef(compare)

  useEffect(() => {
    const prevItems = prevItemsRef.current
    const compareChanged = compareRef.current !== compare
    compareRef.current = compare
    prevItemsRef.current = items

    let isPureAppend = items.length >= prevItems.length
    if (isPureAppend) {
      for (let i = 0; i < prevItems.length; i++) {
        if (prevItems[i] !== items[i]) {
          isPureAppend = false
          break
        }
      }
    }

    if (compareChanged || !isPureAppend) {
      const copy = [...items].sort(compare)
      sortedRef.current = copy
      setSorted(copy)
      return
    }

    if (items.length === prevItems.length) return // nothing new, comparator unchanged

    let next = sortedRef.current
    for (let i = prevItems.length; i < items.length; i++) {
      next = insertSorted(next, items[i], compare)
    }
    sortedRef.current = next
    setSorted(next)
  }, [items, compare])

  return sorted
}
