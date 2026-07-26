import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { SqliScanItemView } from './sqli-scan-item-view'
import type { SqliScanItemResult } from '@/lib/types'

function makeItem(overrides: Partial<SqliScanItemResult> = {}): SqliScanItemResult {
  return {
    history_id: 'req-1',
    method: 'GET',
    url: 'http://example.com/?id=1',
    report: {
      baseline: { status: 200, duration_ms: 10, content_length: 50 },
      findings: [],
      boolean_pairs: [],
      summary: { params_tested: 1, requests: 4, vulnerable: 0, critical: 0 },
    },
    ...overrides,
  }
}

describe('SqliScanItemView', () => {
  it('shows "No signal" when nothing vulnerable was found', () => {
    render(<SqliScanItemView item={makeItem()} onTest={() => {}} />)
    expect(screen.getByText('No signal')).toBeInTheDocument()
  })

  it('shows the signal count badge when vulnerable', () => {
    const item = makeItem({
      report: {
        baseline: { status: 200, duration_ms: 10, content_length: 50 },
        findings: [],
        boolean_pairs: [],
        summary: { params_tested: 1, requests: 4, vulnerable: 2, critical: 1 },
      },
    })
    render(<SqliScanItemView item={item} onTest={() => {}} />)
    expect(screen.getByText('2 signals')).toBeInTheDocument()
  })

  it('expands to show the full report on click, and collapses again', async () => {
    const user = userEvent.setup()
    render(<SqliScanItemView item={makeItem()} onTest={() => {}} />)

    expect(screen.queryByText(/baseline: status 200,\s*10\s*ms, 50 bytes/)).not.toBeInTheDocument()

    await user.click(screen.getByText(/example\.com/))
    expect(screen.getByText(/baseline: status 200,\s*10\s*ms, 50 bytes/)).toBeInTheDocument()

    await user.click(screen.getByLabelText('Collapse'))
    expect(screen.queryByText(/baseline: status 200,\s*10\s*ms, 50 bytes/)).not.toBeInTheDocument()
  })

  it('calls onTest with the item when "Test" is clicked', async () => {
    const user = userEvent.setup()
    const onTest = vi.fn()
    const item = makeItem()
    render(<SqliScanItemView item={item} onTest={onTest} />)

    await user.click(screen.getByRole('button', { name: /Test/ }))
    expect(onTest).toHaveBeenCalledWith(item)
  })
})
