import type { Headers } from './types'

export function headersToText(headers: Headers | undefined): string {
  if (!headers) return ''
  const lines: string[] = []
  for (const [key, values] of Object.entries(headers)) {
    for (const value of values) {
      lines.push(`${key}: ${value}`)
    }
  }
  return lines.join('\n')
}

export function textToHeaders(text: string): Headers {
  const headers: Headers = {}
  for (const rawLine of text.split('\n')) {
    const line = rawLine.trim()
    if (!line) continue
    const idx = line.indexOf(':')
    if (idx === -1) continue
    const key = line.slice(0, idx).trim()
    const value = line.slice(idx + 1).trim()
    if (!key) continue
    if (!headers[key]) headers[key] = []
    headers[key].push(value)
  }
  return headers
}

export function parseJSONHeaders(raw: string): Headers {
  try {
    const parsed = JSON.parse(raw)
    return parsed && typeof parsed === 'object' ? (parsed as Headers) : {}
  } catch {
    return {}
  }
}
