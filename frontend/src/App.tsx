import { lazy, Suspense, useState } from 'react'
import { AnimatePresence, motion, MotionConfig } from 'framer-motion'
import { Loader2 } from 'lucide-react'
import { TooltipProvider } from '@/components/ui/tooltip'
import { AppSidebar, type NavKey } from '@/components/app-sidebar'
import type { RequestSeed } from '@/components/request-editor'
import { headersToText, parseJSONHeaders } from '@/lib/headers'
import { api } from '@/lib/api'
import type { Headers, SendTarget, SendableRequest } from '@/lib/types'
import type { RepeaterSeed } from '@/features/repeater/repeater'
import type { IntruderSeed } from '@/features/intruder/intruder'

// Each tool is its own chunk, loaded on first visit rather than all bundled
// together up front — with 14 of these (several sizable, e.g. the SQLi/XSS
// checkers), eagerly importing every one of them meant a single ~650KB JS
// chunk even though a session typically only ever opens a handful of tabs.
const ProxyHistory = lazy(() =>
  import('@/features/proxy-history/proxy-history').then((m) => ({ default: m.ProxyHistory })),
)
const Intercept = lazy(() =>
  import('@/features/intercept/intercept').then((m) => ({ default: m.Intercept })),
)
const Repeater = lazy(() =>
  import('@/features/repeater/repeater').then((m) => ({ default: m.Repeater })),
)
const Intruder = lazy(() =>
  import('@/features/intruder/intruder').then((m) => ({ default: m.Intruder })),
)
const Decoder = lazy(() => import('@/features/decoder/decoder').then((m) => ({ default: m.Decoder })))
const PortScanner = lazy(() =>
  import('@/features/port-scanner/port-scanner').then((m) => ({ default: m.PortScanner })),
)
const Spider = lazy(() => import('@/features/spider/spider').then((m) => ({ default: m.Spider })))
const PassiveScan = lazy(() =>
  import('@/features/passive-scan/passive-scan').then((m) => ({ default: m.PassiveScan })),
)
const SubdomainFinder = lazy(() =>
  import('@/features/subdomain-finder/subdomain-finder').then((m) => ({ default: m.SubdomainFinder })),
)
const Settings = lazy(() =>
  import('@/features/settings/settings').then((m) => ({ default: m.Settings })),
)
const Certificate = lazy(() =>
  import('@/features/certificate/certificate').then((m) => ({ default: m.Certificate })),
)
const WordPressCheck = lazy(() =>
  import('@/features/wordpress-check/wordpress-check').then((m) => ({ default: m.WordPressCheck })),
)
const SqliCheck = lazy(() =>
  import('@/features/sqli-check/sqli-check').then((m) => ({ default: m.SqliCheck })),
)
const XssCheck = lazy(() =>
  import('@/features/xss-check/xss-check').then((m) => ({ default: m.XssCheck })),
)

const TITLES: Record<NavKey, string> = {
  history: 'Proxy History',
  intercept: 'Intercept',
  repeater: 'Repeater',
  intruder: 'Intruder',
  'wordpress-check': 'WordPress Check',
  'sqli-check': 'SQL Injection',
  'xss-check': 'XSS',
  'port-scanner': 'Port Scanner',
  'subdomain-finder': 'Subdomain Finder',
  spider: 'Spider',
  'passive-scan': 'Passive Scan',
  decoder: 'Decoder',
  certificate: 'Certificate',
  settings: 'Settings',
}

function toHeaders(req: SendableRequest): Headers {
  return typeof req.headers === 'string' ? parseJSONHeaders(req.headers) : req.headers
}

function toSeed(req: SendableRequest): RequestSeed {
  return {
    value: {
      method: req.method,
      url: req.url,
      headersText: headersToText(toHeaders(req)),
      body: req.body,
    },
    nonce: Date.now(),
  }
}

function App() {
  const [nav, setNav] = useState<NavKey>('history')
  const [repeaterSeed, setRepeaterSeed] = useState<RepeaterSeed | null>(null)
  const [intruderSeed, setIntruderSeed] = useState<IntruderSeed | null>(null)
  const [sqliSeed, setSqliSeed] = useState<RequestSeed | null>(null)
  const [xssSeed, setXssSeed] = useState<RequestSeed | null>(null)

  function handleSendTo(target: SendTarget, req: SendableRequest): void {
    if (target === 'repeater') {
      setRepeaterSeed(toSeed(req))
      setNav('repeater')
      return
    }
    if (target === 'intruder') {
      setIntruderSeed(toSeed(req))
      setNav('intruder')
      return
    }
    if (target === 'sqli-check') {
      setSqliSeed(toSeed(req))
      setNav('sqli-check')
      return
    }
    if (target === 'xss-check') {
      setXssSeed(toSeed(req))
      setNav('xss-check')
      return
    }
    // 'intercept-response': resend this exact request through the real proxy
    // pipeline with its response forced into the Intercept > Responses queue
    // — fire-and-forget from here, it resolves once the user forwards/drops
    // it over there (the HTTP call just stays pending until then).
    api
      .repeaterSend(
        { method: req.method, url: req.url, headers: toHeaders(req), body: req.body },
        { interceptResponse: true },
      )
      .catch(() => {
        /* the tab may navigate away before this resolves — nothing to show for it here */
      })
    setNav('intercept')
  }

  function renderPanel(key: NavKey) {
    switch (key) {
      case 'history':
        return <ProxyHistory onSendTo={handleSendTo} />
      case 'intercept':
        return <Intercept onSendTo={handleSendTo} />
      case 'repeater':
        return <Repeater seed={repeaterSeed} />
      case 'intruder':
        return <Intruder seed={intruderSeed} />
      case 'wordpress-check':
        return <WordPressCheck />
      case 'sqli-check':
        return <SqliCheck seed={sqliSeed} />
      case 'xss-check':
        return <XssCheck seed={xssSeed} />
      case 'decoder':
        return <Decoder />
      case 'port-scanner':
        return <PortScanner />
      case 'spider':
        return <Spider />
      case 'passive-scan':
        return <PassiveScan />
      case 'subdomain-finder':
        return <SubdomainFinder />
      case 'settings':
        return <Settings />
      case 'certificate':
        return <Certificate />
    }
  }

  return (
    <MotionConfig reducedMotion="user">
      <TooltipProvider delayDuration={200}>
        <div className="h-screen flex bg-background text-foreground">
          <AppSidebar active={nav} onSelect={setNav} />

          <div className="flex-1 min-w-0 flex flex-col">
            <header className="border-b px-6 py-4 flex items-center gap-3 overflow-hidden">
              <span className="font-mono text-primary text-lg font-semibold tracking-tight select-none">
                nullsec<span className="animate-term-cursor text-primary/80">▌</span>
              </span>
              <span className="h-5 w-px bg-border" />
              <AnimatePresence mode="wait" initial={false}>
                <motion.h1
                  key={nav}
                  initial={{ opacity: 0, y: -6 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: 6 }}
                  transition={{ duration: 0.15, ease: 'easeOut' }}
                  className="text-lg font-semibold tracking-tight"
                >
                  {TITLES[nav]}
                </motion.h1>
              </AnimatePresence>
            </header>

            <main className="flex-1 min-h-0 p-6">
              <AnimatePresence mode="wait" initial={false}>
                <motion.div
                  key={nav}
                  initial={{ opacity: 0, y: 8 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: -8 }}
                  transition={{ duration: 0.15, ease: 'easeOut' }}
                  className="h-full"
                >
                  <Suspense
                    fallback={
                      <div className="h-full flex items-center justify-center text-muted-foreground">
                        <Loader2 className="size-6 animate-spin" />
                      </div>
                    }
                  >
                    {renderPanel(nav)}
                  </Suspense>
                </motion.div>
              </AnimatePresence>
            </main>
          </div>
        </div>
      </TooltipProvider>
    </MotionConfig>
  )
}

export default App
