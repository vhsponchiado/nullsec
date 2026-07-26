import { useState } from 'react'
import { AnimatePresence, motion, MotionConfig } from 'framer-motion'
import { TooltipProvider } from '@/components/ui/tooltip'
import { AppSidebar, type NavKey } from '@/components/app-sidebar'
import { ProxyHistory } from '@/features/proxy-history/proxy-history'
import { Intercept } from '@/features/intercept/intercept'
import { Repeater, type RepeaterSeed } from '@/features/repeater/repeater'
import { Intruder, type IntruderSeed } from '@/features/intruder/intruder'
import { Decoder } from '@/features/decoder/decoder'
import { PortScanner } from '@/features/port-scanner/port-scanner'
import { Spider } from '@/features/spider/spider'
import { PassiveScan } from '@/features/passive-scan/passive-scan'
import { SubdomainFinder } from '@/features/subdomain-finder/subdomain-finder'
import { Settings } from '@/features/settings/settings'
import { Certificate } from '@/features/certificate/certificate'
import { WordPressCheck } from '@/features/wordpress-check/wordpress-check'
import { SqliCheck } from '@/features/sqli-check/sqli-check'
import { XssCheck } from '@/features/xss-check/xss-check'
import type { RequestSeed } from '@/components/request-editor'
import { headersToText, parseJSONHeaders } from '@/lib/headers'
import { api } from '@/lib/api'
import type { Headers, SendTarget, SendableRequest } from '@/lib/types'

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
                  {renderPanel(nav)}
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
