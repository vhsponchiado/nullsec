import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import { XssReportView } from './xss-report-view'
import type { XssReport } from '@/lib/types'

function baseReport(overrides: Partial<XssReport> = {}): XssReport {
  return {
    findings: [],
    summary: { params_tested: 1, requests: 4, vulnerable: 0 },
    ...overrides,
  }
}

describe('XssReportView', () => {
  it('renders the summary badges', () => {
    render(
      <XssReportView report={baseReport({ summary: { params_tested: 2, requests: 8, vulnerable: 1 } })} />,
    )
    expect(screen.getByText('2 parameters')).toBeInTheDocument()
    expect(screen.getByText('8 requests')).toBeInTheDocument()
    expect(screen.getByText('1 signals found')).toBeInTheDocument()
    expect(screen.getByText('possible XSS')).toBeInTheDocument()
  })

  it('shows a hint when no parameters were tested', () => {
    render(<XssReportView report={baseReport({ summary: { params_tested: 0, requests: 0, vulnerable: 0 } })} />)
    expect(screen.getByText(/No parameters found/)).toBeInTheDocument()
  })

  it('groups findings by parameter, labels the context, and flags vulnerable ones', () => {
    render(
      <XssReportView
        report={baseReport({
          findings: [
            {
              parameter: 'q',
              location: 'query',
              payload: '<script>alert(1)</script>',
              context: 'html',
              status: 200,
              content_length: 300,
              vulnerable: true,
              severity: 'critical',
              note: 'Reflected verbatim',
            },
          ],
        })}
      />,
    )
    expect(screen.getByText('<script>alert(1)</script>')).toBeInTheDocument()
    expect(screen.getByText('HTML tag')).toBeInTheDocument()
    expect(screen.getByText('1 signals')).toBeInTheDocument()
  })

  it('shows "No signal" for a parameter group with nothing vulnerable', () => {
    render(
      <XssReportView
        report={baseReport({
          findings: [
            {
              parameter: 'q',
              location: 'query',
              payload: '<script>alert(1)</script>',
              context: 'html',
              status: 200,
              content_length: 300,
              vulnerable: false,
              severity: 'info',
              note: 'Escaped',
            },
          ],
        })}
      />,
    )
    expect(screen.getByText('No signal')).toBeInTheDocument()
  })
})
