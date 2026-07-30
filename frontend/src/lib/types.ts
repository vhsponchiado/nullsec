export type Headers = Record<string, string[]>

export interface HistoryItem {
  id: string
  method: string
  url: string
  host: string
  headers: string
  body: string
  status: number
  resp_headers: string
  resp_body: string
  size: number
  duration_ms: number
  outcome: 'completed' | 'dropped'
  created_at: string
}

// Lightweight row for the history list: no headers/body/resp_body, which
// can be arbitrarily large. The backend's `request_completed`/`request_dropped`
// WS events carry exactly this shape (see entity.Request.Summary on the Go
// side), not the full record, so browsing/crawling doesn't push captured
// bodies over the socket for every request. Fetch the full HistoryItem via
// GET /api/history/{id} once a row is actually selected.
export interface HistorySummary {
  id: string
  method: string
  url: string
  host: string
  status: number
  size: number
  duration_ms: number
  outcome: 'completed' | 'dropped'
  created_at: string
}

export interface HistoryPage {
  items: HistorySummary[]
  total: number
  page: number
  limit: number
}

export interface PendingRequest {
  id: string
  method: string
  url: string
  headers: Headers
  body: string
  created_at: string
}

export interface EditedRequest {
  method: string
  url: string
  headers: Headers
  body: string
}

export interface RepeaterResponse {
  status: number
  headers: string
  body: string
  size: number
  duration_ms: number
  dropped?: boolean
  timed_out?: boolean
  error?: string
}

export interface PendingResponse {
  id: string
  request_method: string
  request_url: string
  status: number
  headers: Headers
  body: string
}

export interface EditedResponse {
  status: number
  headers: Headers
  body: string
}

export type WPCheckSeverity = 'info' | 'warning' | 'critical'

export interface WordPressCheckResult {
  path: string
  url: string
  category: string
  description: string
  status: number
  found: boolean
  severity: WPCheckSeverity
  note: string
  error?: string
}

export interface WordPressCheckSummary {
  checked: number
  found: number
  critical: number
}

export interface WordPressCheckReport {
  results: WordPressCheckResult[]
  summary: WordPressCheckSummary
}

export type SqliSeverity = 'info' | 'warning' | 'critical'
export type SqliPayloadType = 'error' | 'boolean' | 'time'

export interface SqliPayload {
  value: string
  type: SqliPayloadType
}

export interface SqliBaseline {
  status: number
  duration_ms: number
  content_length: number
  error?: string
}

export interface SqliFinding {
  parameter: string
  location: 'query' | 'body'
  payload: string
  payload_type: SqliPayloadType
  status: number
  duration_ms: number
  content_length: number
  vulnerable: boolean
  severity: SqliSeverity
  note: string
  error?: string
}

export interface SqliBooleanPairResult {
  parameter: string
  location: 'query' | 'body'
  true_payload: string
  false_payload: string
  true_status: number
  false_status: number
  true_content_length: number
  false_content_length: number
  vulnerable: boolean
  severity: SqliSeverity
  note: string
  error?: string
}

export interface SqliSummary {
  params_tested: number
  requests: number
  vulnerable: number
  critical: number
}

export interface SqliReport {
  baseline: SqliBaseline
  findings: SqliFinding[]
  boolean_pairs: SqliBooleanPairResult[]
  summary: SqliSummary
}

/** One Proxy History entry's outcome from a batch SQLi Scan run. */
export interface SqliScanItemResult {
  history_id: string
  method: string
  url: string
  report: SqliReport
}

export interface SqliScanStartRequest {
  host?: string
  concurrency?: number
}

export interface SqliScanStartResponse {
  scan_id: string
  total: number
}

export type SqliScanStatus = 'running' | 'done' | 'stopped'

export interface SqliScanSnapshot {
  status: SqliScanStatus
  total: number
  results: SqliScanItemResult[]
}

export interface SqliScanResultEvent {
  scan_id: string
  result: SqliScanItemResult
}

export type WsEventType =
  | 'request_intercepted'
  | 'request_forwarded'
  | 'request_completed'
  | 'request_dropped'
  | 'response_intercepted'
  | 'response_forwarded'
  | 'response_dropped'
  | 'intruder_result'
  | 'intruder_done'
  | 'intruder_stopped'
  | 'portscan_result'
  | 'portscan_done'
  | 'portscan_stopped'
  | 'subdomain_result'
  | 'subdomain_total'
  | 'subdomain_done'
  | 'subdomain_stopped'
  | 'dirfinder_result'
  | 'dirfinder_done'
  | 'dirfinder_stopped'
  | 'spider_page'
  | 'spider_done'
  | 'spider_stopped'
  | 'xssscan_result'
  | 'xssscan_done'
  | 'xssscan_stopped'
  | 'sqliscan_result'
  | 'sqliscan_done'
  | 'sqliscan_stopped'

export interface WsEvent<T = unknown> {
  type: WsEventType
  payload: T
}

export interface DecodeOperation {
  key: string
  label: string
  hash_only: boolean
}

export interface DecodeSmartStep {
  op: string
  label: string
  output: string
}

export type IntruderAttackType = 'sniper' | 'battering_ram' | 'pitchfork' | 'cluster_bomb'

export interface IntruderHeaderPair {
  key: string
  value: string
}

export interface IntruderTemplate {
  method: string
  url: string
  headers: IntruderHeaderPair[]
  body: string
}

export interface IntruderStartRequest {
  template: IntruderTemplate
  attack_type: IntruderAttackType
  payload_sets: string[][]
  grep?: string
  concurrency?: number
  timeout_ms?: number
  rate_per_sec?: number
}

export interface IntruderStartResponse {
  attack_id: string
  total: number
}

export interface IntruderResultSummary {
  index: number
  payloads: string[]
  status: number
  length: number
  duration_ms: number
  match?: boolean
  error?: string
}

export interface IntruderResultDetail extends IntruderResultSummary {
  request_method: string
  request_url: string
  request_headers: string
  request_body: string
  response_headers: string
  response_body: string
}

export type IntruderStatus = 'running' | 'done' | 'stopped'

export interface IntruderSnapshot {
  status: IntruderStatus
  total: number
  results: IntruderResultSummary[]
}

export interface IntruderResultEvent {
  attack_id: string
  result: IntruderResultSummary
}

export interface IntruderDoneEvent {
  attack_id: string
}

export interface PortInfo {
  port: number
  service: string
}

export type PortScanState = 'open' | 'closed' | 'filtered'

export interface PortScanResult {
  port: number
  state: PortScanState
  service: string
  banner?: string
  duration_ms: number
}

export interface PortScanStartRequest {
  host: string
  ports: number[]
  timeout_ms?: number
  concurrency?: number
}

export interface PortScanStartResponse {
  scan_id: string
  total: number
}

export type PortScanStatus = 'running' | 'done' | 'stopped'

export interface PortScanSnapshot {
  status: PortScanStatus
  total: number
  results: PortScanResult[]
}

export interface PortScanPresets {
  top100: PortInfo[]
  top1000: PortInfo[]
}

export interface PortScanResultEvent {
  scan_id: string
  result: PortScanResult
}

export interface SubdomainResult {
  subdomain: string
  // False for a keyword/broad-search hit that isn't actually the target
  // domain or a subdomain of it — just a similarly-named, distinct apex
  // domain (see subdomain.go's package doc comment).
  same_root: boolean
  resolved: boolean
  ips?: string[]
  sources: string[]
  duration_ms: number
}

export interface SubdomainStartRequest {
  domain: string
  use_crtsh: boolean
  // Queries HackerTarget's free hostsearch API — an independent passive DNS
  // data source from crt.sh, so it finds different names and keeps working
  // when crt.sh itself is down.
  use_hackertarget?: boolean
  wordlist_size: number
  // If non-empty, additionally runs a broad crt.sh search for this keyword
  // — independent of use_crtsh/wordlist_size, since it finds unrelated
  // "sibling" domains rather than subdomains.
  crtsh_keyword?: string
  // If > 0, additionally brute-forces Wordlist prefixes joined with "-"
  // instead of "." (qa-domain.com instead of qa.domain.com) — guesses
  // sibling apex domains via DNS instead of certificate transparency, for
  // when the sibling was never issued a publicly-trusted cert at all.
  sibling_wordlist_size?: number
  timeout_ms?: number
  concurrency?: number
}

export interface SubdomainStartResponse {
  scan_id: string
  total: number
}

export type SubdomainStatus = 'running' | 'done' | 'stopped'

export interface SubdomainSnapshot {
  status: SubdomainStatus
  total: number
  results: SubdomainResult[]
  warning?: string
}

export interface SubdomainResultEvent {
  scan_id: string
  result: SubdomainResult
}

export interface SubdomainTotalEvent {
  scan_id: string
  total: number
}

export interface DirFinderResult {
  path: string
  url: string
  status: number
  size: number
  redirect?: string
  duration_ms: number
  error?: string
}

export interface DirFinderStartRequest {
  target: string
  wordlist_size: number
  extensions?: string[]
  concurrency?: number
  timeout_ms?: number
}

export interface DirFinderStartResponse {
  scan_id: string
  total: number
}

export type DirFinderStatus = 'running' | 'done' | 'stopped'

export interface DirFinderSnapshot {
  status: DirFinderStatus
  total: number
  results: DirFinderResult[]
}

export interface DirFinderResultEvent {
  scan_id: string
  result: DirFinderResult
}

export interface SpiderForm {
  action: string
  method: string
  inputs: string[]
}

export interface SpiderPage {
  url: string
  status: number
  content_type: string
  title?: string
  size: number
  duration_ms: number
  depth: number
  links_found: number
  forms?: SpiderForm[]
  error?: string
}

export interface SpiderStartRequest {
  url: string
  max_depth?: number
  max_pages?: number
  include_subdomains?: boolean
  concurrency?: number
}

export interface SpiderStartResponse {
  spider_id: string
}

export type SpiderStatus = 'running' | 'done' | 'stopped'

export interface SpiderSnapshot {
  status: SpiderStatus
  pages: SpiderPage[]
}

export interface SpiderPageEvent {
  spider_id: string
  page: SpiderPage
}

export type PassiveSeverity = 'info' | 'warning' | 'critical'

export interface PassiveFinding {
  history_id: string
  method: string
  url: string
  host: string
  category: string
  title: string
  severity: PassiveSeverity
  detail: string
}

export interface PassiveSummary {
  requests_scanned: number
  findings: number
  critical: number
  warning: number
}

export interface PassiveReport {
  findings: PassiveFinding[]
  summary: PassiveSummary
}

export type XssSeverity = 'info' | 'critical'
export type XssContext = 'html' | 'attribute' | 'js' | 'url'

export interface XssPayload {
  value: string
  context: XssContext
}

export interface XssFinding {
  parameter: string
  location: 'query' | 'body'
  payload: string
  context: XssContext
  status: number
  content_length: number
  vulnerable: boolean
  severity: XssSeverity
  note: string
  error?: string
}

export interface XssSummary {
  params_tested: number
  requests: number
  vulnerable: number
}

export interface XssReport {
  findings: XssFinding[]
  summary: XssSummary
  error?: string
}

/** One Proxy History entry's outcome from a batch XSS Scan run. */
export interface XssScanItemResult {
  history_id: string
  method: string
  url: string
  report: XssReport
}

export interface XssScanStartRequest {
  host?: string
  concurrency?: number
}

export interface XssScanStartResponse {
  scan_id: string
  total: number
}

export type XssScanStatus = 'running' | 'done' | 'stopped'

export interface XssScanSnapshot {
  status: XssScanStatus
  total: number
  results: XssScanItemResult[]
}

export interface XssScanResultEvent {
  scan_id: string
  result: XssScanItemResult
}

export type UpstreamProxyType = 'none' | 'socks5' | 'http'

export interface UpstreamProxyConfig {
  type: UpstreamProxyType
  address: string
  username?: string
  password?: string
}

/** Where SendToMenu (and App's handleSendTo) can route a captured request. */
export type SendTarget = 'repeater' | 'intruder' | 'sqli-check' | 'xss-check' | 'intercept-response'

/** The common shape SendToMenu needs from a request, regardless of which
 * feature captured it (Proxy History, Intercept, Repeater, ...). */
export interface SendableRequest {
  method: string
  url: string
  // HistoryItem/PendingResponse store headers as a raw JSON string; the
  // Intercept "pending request" list already has them pre-parsed. Accept
  // either so callers don't need to care which shape their data is in.
  headers: string | Headers
  body: string
}

/** The two halves of the built-in username:password wordlist, in matching
 * order: pairing usernames[i] with passwords[i] via Intruder's Pitchfork
 * attack reproduces the original credential pairs. */
export interface WordlistCredentials {
  usernames: string[]
  passwords: string[]
}
