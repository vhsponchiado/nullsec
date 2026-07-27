import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuTrigger,
} from '@/components/ui/context-menu'
import { Send, Syringe, Radio, Crosshair, Code2 } from 'lucide-react'
import type { SendTarget, SendableRequest } from '@/lib/types'

interface SendToMenuProps {
  // A plain object when the caller already has the full request in hand
  // (e.g. Intercept's pending queue); a loader function when it doesn't
  // (e.g. a Proxy History row, which only holds the lightweight summary;
  // headers/body get fetched on demand only if a menu item is actually
  // clicked, instead of eagerly for every row in the table).
  request: SendableRequest | (() => Promise<SendableRequest>)
  onSendTo: (target: SendTarget, req: SendableRequest) => void
  children: ReactNode
  // "Intercept response" resends the request through the proxy to force-
  // capture a fresh response: that's a deliberate replay for an already-
  // completed history entry, but nonsensical (and a real duplicate outbound
  // request) for a request that's still sitting unresolved in the Intercept
  // queue, since the original hasn't gone out yet either.
  allowInterceptResponse?: boolean
}

/** Wraps `children` in a right-click context menu with "send this request
 * to..." shortcuts (Repeater, SQL Injection); the headers (cookies, auth,
 * etc.) travel with it automatically since they come straight from the
 * captured request. */
export function SendToMenu({
  request,
  onSendTo,
  children,
  allowInterceptResponse = true,
}: SendToMenuProps) {
  const { t } = useTranslation('request-components')
  async function send(target: SendTarget) {
    const req = typeof request === 'function' ? await request() : request
    onSendTo(target, req)
  }

  return (
    <ContextMenu>
      <ContextMenuTrigger asChild>{children}</ContextMenuTrigger>
      <ContextMenuContent>
        <ContextMenuItem onClick={() => send('repeater')}>
          <Send />
          {t('send_to_menu_repeater')}
        </ContextMenuItem>
        <ContextMenuItem onClick={() => send('intruder')}>
          <Crosshair />
          {t('send_to_menu_intruder')}
        </ContextMenuItem>
        <ContextMenuItem onClick={() => send('sqli-check')}>
          <Syringe />
          {t('send_to_menu_sqli')}
        </ContextMenuItem>
        <ContextMenuItem onClick={() => send('xss-check')}>
          <Code2 />
          {t('send_to_menu_xss')}
        </ContextMenuItem>
        {allowInterceptResponse && (
          <ContextMenuItem onClick={() => send('intercept-response')}>
            <Radio />
            {t('send_to_menu_intercept_response')}
          </ContextMenuItem>
        )}
      </ContextMenuContent>
    </ContextMenu>
  )
}
