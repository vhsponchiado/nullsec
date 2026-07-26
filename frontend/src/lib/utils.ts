import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

/** Buckets items by keyFn into an array of [key, items] pairs, in
 * first-seen key order (not insertion-into-Map order, which for a Map
 * matches this anyway, but callers rely on it so it's worth stating). Used
 * to group findings/payloads by parameter, category, or type across the
 * scanner result tables. */
export function groupBy<T>(items: T[], keyFn: (item: T) => string): Array<[string, T[]]> {
  const order: string[] = []
  const groups = new Map<string, T[]>()
  for (const item of items) {
    const key = keyFn(item)
    let group = groups.get(key)
    if (!group) {
      group = []
      groups.set(key, group)
      order.push(key)
    }
    group.push(item)
  }
  return order.map((key) => [key, groups.get(key)!])
}
