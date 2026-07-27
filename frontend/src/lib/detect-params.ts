export interface DetectedParam {
  location: 'query' | 'body'
  name: string
}

/** Finds candidate injection points: every query-string key in `url`, plus
 * every key in `body` when it parses as form-urlencoded. Used by the SQLi
 * and XSS manual-injection pickers to know what's available to target. */
export function detectParams(url: string, body: string): DetectedParam[] {
  const out: DetectedParam[] = []
  try {
    const u = new URL(url)
    for (const name of u.searchParams.keys()) out.push({ location: 'query', name })
  } catch {
    // invalid/incomplete URL: no query params yet
  }
  if (body.trim()) {
    try {
      const params = new URLSearchParams(body)
      for (const name of params.keys()) out.push({ location: 'body', name })
    } catch {
      // not form-urlencoded, so ignore
    }
  }
  return out
}
