import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { XssScanItemView } from './xss-scan-item-view'
import type { XssScanItemResult } from '@/lib/types'

function makeItem(overrides: Partial<XssScanItemResult> = {}): XssScanItemResult {
  return {
    history_id: 'req-1',
    method: 'GET',
    url: 'http://example.com/?q=test',
    report: {
      findings: [],
      summary: { params_tested: 1, requests: 4, vulnerable: 0 },
    },
    ...overrides,
  }
}

describe('XssScanItemView', () => {
  it('shows "No signal" when nothing vulnerable was found', () => {
    render(<XssScanItemView item={makeItem()} onTest={() => {}} />)
    expect(screen.getByText('No signal')).toBeInTheDocument()
  })

  it('shows the signal count badge when vulnerable', () => {
    const item = makeItem({
      report: { findings: [], summary: { params_tested: 1, requests: 4, vulnerable: 3 } },
    })
    render(<XssScanItemView item={item} onTest={() => {}} />)
    expect(screen.getByText('3 signals')).toBeInTheDocument()
  })

  it('expands to show the full report on click, and collapses again', async () => {
    const user = userEvent.setup()
    render(<XssScanItemView item={makeItem()} onTest={() => {}} />)

    expect(screen.queryByText('1 parameters')).not.toBeInTheDocument()

    await user.click(screen.getByText(/example\.com/))
    expect(screen.getByText('1 parameters')).toBeInTheDocument()

    await user.click(screen.getByLabelText('Collapse'))
    expect(screen.queryByText('1 parameters')).not.toBeInTheDocument()
  })

  it('calls onTest with the item when "Test" is clicked', async () => {
    const user = userEvent.setup()
    const onTest = vi.fn()
    const item = makeItem()
    render(<XssScanItemView item={item} onTest={onTest} />)

    await user.click(screen.getByRole('button', { name: /Test/ }))
    expect(onTest).toHaveBeenCalledWith(item)
  })
})
