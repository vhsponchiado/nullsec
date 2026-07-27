import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { HeadersEditor, type HeaderRow } from '@/components/headers-editor'

const METHODS = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE']

interface TargetRequestFormProps {
  idPrefix: string
  method: string
  onMethodChange: (v: string) => void
  url: string
  onUrlChange: (v: string) => void
  urlPlaceholder: string
  body: string
  onBodyChange: (v: string) => void
  bodyPlaceholder: string
  headerRows: HeaderRow[]
  onHeaderRowsChange: (rows: HeaderRow[]) => void
  /** Rendered next to the URL field: the mode-specific "Scan" button, when
   * there is one (only Automatic mode has one; Manual doesn't). */
  urlAction?: ReactNode
}

/** The "Target request" block (method/URL/body/headers) shared by the SQLi
 * and XSS checkers' Automatic and Manual modes; within one checker, both
 * modes operate on the same in-flight target, so its state lives in the
 * container, not duplicated per mode. */
export function TargetRequestForm({
  idPrefix,
  method,
  onMethodChange,
  url,
  onUrlChange,
  urlPlaceholder,
  body,
  onBodyChange,
  bodyPlaceholder,
  headerRows,
  onHeaderRowsChange,
  urlAction,
}: TargetRequestFormProps) {
  const { t } = useTranslation('request-components')
  return (
    <div className="flex flex-col gap-4 border rounded-lg p-4">
      <h3 className="text-sm font-medium text-muted-foreground">{t('target_request_form_title')}</h3>

      <div className="flex gap-2 items-end">
        <div className="flex flex-col gap-1.5">
          <Label htmlFor={`${idPrefix}-method`}>{t('target_request_form_method')}</Label>
          <Select value={method} onValueChange={onMethodChange}>
            <SelectTrigger id={`${idPrefix}-method`} size="lg" className="w-28">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {METHODS.map((m) => (
                <SelectItem key={m} value={m}>
                  {m}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="flex flex-col gap-1.5 flex-1">
          <Label htmlFor={`${idPrefix}-url`}>{t('target_request_form_url_label')}</Label>
          <Input
            id={`${idPrefix}-url`}
            size="lg"
            className="font-mono"
            value={url}
            onChange={(e) => onUrlChange(e.target.value)}
            placeholder={urlPlaceholder}
          />
        </div>
        {urlAction}
      </div>

      <div className="flex flex-col gap-1.5">
        <Label htmlFor={`${idPrefix}-body`}>
          {t('target_request_form_body_label', { example: bodyPlaceholder })}
        </Label>
        <Textarea
          id={`${idPrefix}-body`}
          className="font-mono text-sm min-h-20 resize-y"
          value={body}
          onChange={(e) => onBodyChange(e.target.value)}
          placeholder={bodyPlaceholder}
        />
      </div>

      <div className="flex flex-col gap-1.5">
        <Label>{t('target_request_form_headers_label')}</Label>
        <HeadersEditor rows={headerRows} onChange={onHeaderRowsChange} />
      </div>
    </div>
  )
}
