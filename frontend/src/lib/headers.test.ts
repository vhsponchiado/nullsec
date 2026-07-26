import { describe, expect, it } from 'vitest'
import { headersToText, textToHeaders, parseJSONHeaders } from './headers'
import type { Headers } from './types'

describe('headersToText', () => {
  it('returns an empty string for undefined', () => {
    expect(headersToText(undefined)).toBe('')
  })

  it('renders one "Key: value" line per value', () => {
    const headers: Headers = { 'Content-Type': ['application/json'], 'X-Multi': ['a', 'b'] }
    expect(headersToText(headers)).toBe('Content-Type: application/json\nX-Multi: a\nX-Multi: b')
  })
})

describe('textToHeaders', () => {
  it('parses "Key: value" lines, grouping repeated keys', () => {
    const text = 'Content-Type: application/json\nX-Multi: a\nX-Multi: b'
    expect(textToHeaders(text)).toEqual({
      'Content-Type': ['application/json'],
      'X-Multi': ['a', 'b'],
    })
  })

  it('trims whitespace around keys and values', () => {
    expect(textToHeaders('  Host  :   example.com  ')).toEqual({ Host: ['example.com'] })
  })

  it('skips blank lines and lines without a colon', () => {
    expect(textToHeaders('Host: example.com\n\nnot-a-header\nAccept: */*')).toEqual({
      Host: ['example.com'],
      Accept: ['*/*'],
    })
  })

  it('preserves a colon that is part of the value (e.g. a URL)', () => {
    expect(textToHeaders('Location: https://example.com/path')).toEqual({
      Location: ['https://example.com/path'],
    })
  })

  it('round-trips through headersToText', () => {
    const headers: Headers = { Host: ['example.com'], Accept: ['*/*'] }
    expect(textToHeaders(headersToText(headers))).toEqual(headers)
  })
})

describe('parseJSONHeaders', () => {
  it('parses a valid JSON object', () => {
    expect(parseJSONHeaders('{"Host":["example.com"]}')).toEqual({ Host: ['example.com'] })
  })

  it('returns {} for invalid JSON', () => {
    expect(parseJSONHeaders('not json')).toEqual({})
  })

  it('returns {} for JSON that is not an object (e.g. a bare number)', () => {
    expect(parseJSONHeaders('42')).toEqual({})
  })

  it('returns {} for JSON null', () => {
    expect(parseJSONHeaders('null')).toEqual({})
  })
})
