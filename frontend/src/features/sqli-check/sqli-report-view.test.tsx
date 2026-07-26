import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import { SqliReportView } from './sqli-report-view'
import type { SqliReport } from '@/lib/types'

function baseReport(overrides: Partial<SqliReport> = {}): SqliReport {
  return {
    baseline: { status: 200, duration_ms: 50, content_length: 100 },
    findings: [],
    boolean_pairs: [],
    summary: { params_tested: 1, requests: 4, vulnerable: 0, critical: 0 },
    ...overrides,
  }
}

describe('SqliReportView', () => {
  it('renders the summary badges', () => {
    render(
      <SqliReportView
        report={baseReport({ summary: { params_tested: 2, requests: 8, vulnerable: 3, critical: 1 } })}
      />,
    )
    expect(screen.getByText('2 parameters')).toBeInTheDocument()
    expect(screen.getByText('8 requests')).toBeInTheDocument()
    expect(screen.getByText('3 signals found')).toBeInTheDocument()
    expect(screen.getByText('1 critical')).toBeInTheDocument()
  })

  it('shows a hint when no parameters were tested', () => {
    render(<SqliReportView report={baseReport({ summary: { params_tested: 0, requests: 0, vulnerable: 0, critical: 0 } })} />)
    expect(screen.getByText(/No parameters found/)).toBeInTheDocument()
  })

  it('groups findings by parameter and flags vulnerable ones', () => {
    render(
      <SqliReportView
        report={baseReport({
          findings: [
            {
              parameter: 'id',
              location: 'query',
              payload: "' OR 1=1--",
              payload_type: 'error',
              status: 500,
              duration_ms: 40,
              content_length: 120,
              vulnerable: true,
              severity: 'critical',
              note: 'DB error triggered',
            },
          ],
        })}
      />,
    )
    expect(screen.getByText('id', { selector: 'span.font-mono' })).toBeInTheDocument()
    expect(screen.getByText("' OR 1=1--")).toBeInTheDocument()
    expect(screen.getByText('1 signals')).toBeInTheDocument()
  })

  it('renders the true/false boolean-pair section when present', () => {
    render(
      <SqliReportView
        report={baseReport({
          boolean_pairs: [
            {
              parameter: 'id',
              location: 'query',
              true_payload: '1 OR 1=1',
              false_payload: '1 AND 1=2',
              true_status: 200,
              false_status: 200,
              true_content_length: 500,
              false_content_length: 10,
              vulnerable: true,
              severity: 'critical',
              note: 'Response length differs sharply',
            },
          ],
        })}
      />,
    )
    expect(screen.getByText('True/false test')).toBeInTheDocument()
    expect(screen.getByText('Possible SQL injection')).toBeInTheDocument()
  })
})
