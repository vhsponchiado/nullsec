import { describe, expect, it } from 'vitest'
import { statusText, statusTone } from './http-status'

describe('statusText', () => {
  it('returns the known reason phrase for a listed status', () => {
    expect(statusText(200)).toBe('OK')
    expect(statusText(404)).toBe('Not Found')
    expect(statusText(500)).toBe('Internal Server Error')
  })

  it('returns an empty string for an unlisted status', () => {
    expect(statusText(999)).toBe('')
  })
})

describe('statusTone', () => {
  it('treats 0 (no response / dropped) as neutral', () => {
    expect(statusTone(0)).toBe('neutral')
  })

  it('treats 2xx and 3xx as success', () => {
    expect(statusTone(200)).toBe('success')
    expect(statusTone(301)).toBe('success')
    expect(statusTone(399)).toBe('success')
  })

  it('treats 4xx as warning', () => {
    expect(statusTone(400)).toBe('warning')
    expect(statusTone(404)).toBe('warning')
    expect(statusTone(499)).toBe('warning')
  })

  it('treats 5xx (and anything else >= 500) as error', () => {
    expect(statusTone(500)).toBe('error')
    expect(statusTone(503)).toBe('error')
  })
})
