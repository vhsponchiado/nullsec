/** Returns a new array with `item` inserted at the position that keeps
 * `sorted` ordered per `compare`: binary search for the position, O(log n),
 * plus the O(n) splice/copy insertion itself already requires. Used to grow
 * an already-sorted, live-streaming list one arrival at a time without
 * re-sorting the whole thing from scratch on every single new item. */
export function insertSorted<T>(sorted: T[], item: T, compare: (a: T, b: T) => number): T[] {
  const next = sorted.slice()
  let lo = 0
  let hi = next.length
  while (lo < hi) {
    const mid = (lo + hi) >>> 1
    if (compare(next[mid], item) < 0) lo = mid + 1
    else hi = mid
  }
  next.splice(lo, 0, item)
  return next
}
