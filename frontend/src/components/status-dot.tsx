import { useTranslation } from 'react-i18next'
import { statusText, statusTone } from '@/lib/http-status'

const TONE_CLASS: Record<string, string> = {
  success: 'bg-success',
  warning: 'bg-warning',
  error: 'bg-destructive',
  neutral: 'bg-muted-foreground',
}

interface StatusDotProps {
  status: number
}

export function StatusDot({ status }: StatusDotProps) {
  const { t } = useTranslation('request-components')
  const tone = statusTone(status)
  const text = statusText(status)
  return (
    <span className="inline-flex items-center gap-2 font-mono text-sm">
      <span className={`inline-block size-2 rounded-full ${TONE_CLASS[tone]}`} />
      {status === 0 ? t('status_dot_na') : status}
      {text && <span className="text-muted-foreground">{text}</span>}
    </span>
  )
}
