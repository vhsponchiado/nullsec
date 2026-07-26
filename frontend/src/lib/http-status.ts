const STATUS_TEXT: Record<number, string> = {
  200: 'OK',
  201: 'Created',
  204: 'No Content',
  301: 'Moved Permanently',
  302: 'Found',
  304: 'Not Modified',
  400: 'Bad Request',
  401: 'Unauthorized',
  403: 'Forbidden',
  404: 'Not Found',
  405: 'Method Not Allowed',
  429: 'Too Many Requests',
  500: 'Internal Server Error',
  502: 'Bad Gateway',
  503: 'Service Unavailable',
  504: 'Gateway Timeout',
}

export function statusText(status: number): string {
  return STATUS_TEXT[status] ?? ''
}

export type StatusTone = 'success' | 'warning' | 'error' | 'neutral'

export function statusTone(status: number): StatusTone {
  if (status === 0) return 'neutral'
  if (status >= 200 && status < 400) return 'success'
  if (status >= 400 && status < 500) return 'warning'
  return 'error'
}
