import type {
  DecodeOperation,
  DecodeSmartStep,
  EditedRequest,
  EditedResponse,
  Headers,
  HistoryItem,
  HistoryPage,
  IntruderResultDetail,
  IntruderSnapshot,
  IntruderStartRequest,
  IntruderStartResponse,
  PendingRequest,
  PendingResponse,
  PassiveReport,
  PortScanPresets,
  PortScanSnapshot,
  PortScanStartRequest,
  PortScanStartResponse,
  RepeaterResponse,
  SqliPayload,
  SqliReport,
  SqliScanSnapshot,
  SqliScanStartRequest,
  SqliScanStartResponse,
  SpiderSnapshot,
  SpiderStartRequest,
  SpiderStartResponse,
  SubdomainSnapshot,
  SubdomainStartRequest,
  SubdomainStartResponse,
  UpstreamProxyConfig,
  WordlistCredentials,
  WordPressCheckReport,
  XssPayload,
  XssReport,
  XssScanSnapshot,
  XssScanStartRequest,
  XssScanStartResponse,
} from './types'

const API_BASE = import.meta.env.VITE_API_BASE ?? 'http://localhost:8081'

export const caCertUrl = `${API_BASE}/api/ca/cert`

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  })
  if (!res.ok) {
    const text = await res.text().catch(() => res.statusText)
    throw new Error(`${res.status}: ${text}`)
  }
  if (res.status === 204) return undefined as T
  return res.json() as Promise<T>
}

export interface HistoryFilter {
  host?: string
  method?: string
  status?: number
  page?: number
  limit?: number
}

export const api = {
  listHistory(filter: HistoryFilter = {}): Promise<HistoryPage> {
    const params = new URLSearchParams()
    if (filter.host) params.set('host', filter.host)
    if (filter.method) params.set('method', filter.method)
    if (filter.status) params.set('status', String(filter.status))
    if (filter.page) params.set('page', String(filter.page))
    if (filter.limit) params.set('limit', String(filter.limit))
    const qs = params.toString()
    return request(`/api/history${qs ? `?${qs}` : ''}`)
  },

  getHistoryItem(id: string): Promise<HistoryItem> {
    return request(`/api/history/${id}`)
  },

  clearHistory(): Promise<{ status: string }> {
    return request('/api/history', { method: 'DELETE' })
  },

  getInterceptStatus(): Promise<{ enabled: boolean }> {
    return request('/api/intercept/status')
  },

  toggleIntercept(enabled: boolean): Promise<{ enabled: boolean }> {
    return request('/api/intercept/toggle', {
      method: 'POST',
      body: JSON.stringify({ enabled }),
    })
  },

  listPending(): Promise<PendingRequest[]> {
    return request('/api/intercept/pending')
  },

  forwardIntercepted(id: string, edited: EditedRequest): Promise<{ status: string }> {
    return request(`/api/intercept/${id}/forward`, {
      method: 'POST',
      body: JSON.stringify(edited),
    })
  },

  dropIntercepted(id: string): Promise<{ status: string }> {
    return request(`/api/intercept/${id}/drop`, { method: 'POST' })
  },

  repeaterSend(
    edited: EditedRequest,
    opts?: { interceptResponse?: boolean },
  ): Promise<RepeaterResponse> {
    return request('/api/repeater/send', {
      method: 'POST',
      body: JSON.stringify({ ...edited, interceptResponse: opts?.interceptResponse ?? false }),
    })
  },

  getResponseInterceptStatus(): Promise<{ enabled: boolean }> {
    return request('/api/intercept/response/status')
  },

  toggleResponseIntercept(enabled: boolean): Promise<{ enabled: boolean }> {
    return request('/api/intercept/response/toggle', {
      method: 'POST',
      body: JSON.stringify({ enabled }),
    })
  },

  listPendingResponses(): Promise<PendingResponse[]> {
    return request('/api/intercept/response/pending')
  },

  forwardInterceptedResponse(id: string, edited: EditedResponse): Promise<{ status: string }> {
    return request(`/api/intercept/response/${id}/forward`, {
      method: 'POST',
      body: JSON.stringify(edited),
    })
  },

  dropInterceptedResponse(id: string): Promise<{ status: string }> {
    return request(`/api/intercept/response/${id}/drop`, { method: 'POST' })
  },

  regenerateCA(): Promise<{ status: string }> {
    return request('/api/ca/regenerate', { method: 'POST' })
  },

  wordpressCheck(url: string): Promise<WordPressCheckReport> {
    return request('/api/tools/wordpress-check', {
      method: 'POST',
      body: JSON.stringify({ url }),
    })
  },

  sqliCheck(params: {
    method: string
    url: string
    body?: string
    headers?: Headers
  }): Promise<SqliReport> {
    return request('/api/tools/sqli-check', {
      method: 'POST',
      body: JSON.stringify(params),
    })
  },

  listSqliPayloads(): Promise<SqliPayload[]> {
    return request('/api/tools/sqli-payloads')
  },

  listDecodeOperations(): Promise<DecodeOperation[]> {
    return request('/api/tools/decode-operations')
  },

  decode(op: string, mode: 'encode' | 'decode', input: string): Promise<{ output?: string; error?: string }> {
    return request('/api/tools/decode', {
      method: 'POST',
      body: JSON.stringify({ op, mode, input }),
    })
  },

  smartDecode(input: string): Promise<{ steps: DecodeSmartStep[] }> {
    return request('/api/tools/decode/smart', {
      method: 'POST',
      body: JSON.stringify({ input }),
    })
  },

  listPasswordWordlist(): Promise<string[]> {
    return request('/api/tools/wordlist/passwords')
  },

  listUsernameWordlist(): Promise<string[]> {
    return request('/api/tools/wordlist/usernames')
  },

  listCredentialWordlist(): Promise<WordlistCredentials> {
    return request('/api/tools/wordlist/credentials')
  },

  startIntruderAttack(req: IntruderStartRequest): Promise<IntruderStartResponse> {
    return request('/api/tools/intruder/start', {
      method: 'POST',
      body: JSON.stringify(req),
    })
  },

  stopIntruderAttack(id: string): Promise<{ status: string }> {
    return request(`/api/tools/intruder/${id}/stop`, { method: 'POST' })
  },

  getIntruderAttack(id: string): Promise<IntruderSnapshot> {
    return request(`/api/tools/intruder/${id}`)
  },

  getIntruderResult(id: string, index: number): Promise<IntruderResultDetail> {
    return request(`/api/tools/intruder/${id}/results/${index}`)
  },

  portscanPresets(): Promise<PortScanPresets> {
    return request('/api/tools/portscan/presets')
  },

  startPortScan(req: PortScanStartRequest): Promise<PortScanStartResponse> {
    return request('/api/tools/portscan/start', {
      method: 'POST',
      body: JSON.stringify(req),
    })
  },

  stopPortScan(id: string): Promise<{ status: string }> {
    return request(`/api/tools/portscan/${id}/stop`, { method: 'POST' })
  },

  getPortScan(id: string): Promise<PortScanSnapshot> {
    return request(`/api/tools/portscan/${id}`)
  },

  subdomainWordlistSize(): Promise<{ size: number }> {
    return request('/api/tools/subdomain/wordlist-size')
  },

  startSubdomainScan(req: SubdomainStartRequest): Promise<SubdomainStartResponse> {
    return request('/api/tools/subdomain/start', {
      method: 'POST',
      body: JSON.stringify(req),
    })
  },

  stopSubdomainScan(id: string): Promise<{ status: string }> {
    return request(`/api/tools/subdomain/${id}/stop`, { method: 'POST' })
  },

  getSubdomainScan(id: string): Promise<SubdomainSnapshot> {
    return request(`/api/tools/subdomain/${id}`)
  },

  getUpstreamProxy(): Promise<UpstreamProxyConfig> {
    return request('/api/settings/upstream-proxy')
  },

  setUpstreamProxy(config: UpstreamProxyConfig): Promise<UpstreamProxyConfig> {
    return request('/api/settings/upstream-proxy', {
      method: 'POST',
      body: JSON.stringify(config),
    })
  },

  listXssPayloads(): Promise<XssPayload[]> {
    return request('/api/tools/xss-payloads')
  },

  xssCheck(params: {
    method: string
    url: string
    body?: string
    headers?: Headers
  }): Promise<XssReport> {
    return request('/api/tools/xss-check', {
      method: 'POST',
      body: JSON.stringify(params),
    })
  },

  startXssScan(req: XssScanStartRequest): Promise<XssScanStartResponse> {
    return request('/api/tools/xss-scan/start', {
      method: 'POST',
      body: JSON.stringify(req),
    })
  },

  stopXssScan(id: string): Promise<{ status: string }> {
    return request(`/api/tools/xss-scan/${id}/stop`, { method: 'POST' })
  },

  getXssScan(id: string): Promise<XssScanSnapshot> {
    return request(`/api/tools/xss-scan/${id}`)
  },

  startSqliScan(req: SqliScanStartRequest): Promise<SqliScanStartResponse> {
    return request('/api/tools/sqli-scan/start', {
      method: 'POST',
      body: JSON.stringify(req),
    })
  },

  stopSqliScan(id: string): Promise<{ status: string }> {
    return request(`/api/tools/sqli-scan/${id}/stop`, { method: 'POST' })
  },

  getSqliScan(id: string): Promise<SqliScanSnapshot> {
    return request(`/api/tools/sqli-scan/${id}`)
  },

  startSpider(req: SpiderStartRequest): Promise<SpiderStartResponse> {
    return request('/api/tools/spider/start', {
      method: 'POST',
      body: JSON.stringify(req),
    })
  },

  stopSpider(id: string): Promise<{ status: string }> {
    return request(`/api/tools/spider/${id}/stop`, { method: 'POST' })
  },

  getSpider(id: string): Promise<SpiderSnapshot> {
    return request(`/api/tools/spider/${id}`)
  },

  passiveScan(filter: { host?: string } = {}): Promise<PassiveReport> {
    const params = new URLSearchParams()
    if (filter.host) params.set('host', filter.host)
    const qs = params.toString()
    return request(`/api/tools/passive-scan${qs ? `?${qs}` : ''}`)
  },
}
