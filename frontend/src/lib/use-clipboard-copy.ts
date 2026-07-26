import { useState } from 'react'

/** How long "copied!" feedback stays visible before reverting. */
const COPY_FEEDBACK_MS = 1500

/** Tracks which of possibly several copy buttons on a page was just
 * clicked, so its "copied!" state can revert on its own after a beat
 * without stepping on any other button's state. `copiedValue` is whatever
 * key you passed to `copy` — compare it to a button's own key to decide
 * whether to render its "copied" look. */
export function useClipboardCopy<T>() {
  const [copiedValue, setCopiedValue] = useState<T | null>(null)

  async function copy(key: T, text: string) {
    try {
      await navigator.clipboard.writeText(text)
      setCopiedValue(key)
      setTimeout(() => setCopiedValue((cur) => (cur === key ? null : cur)), COPY_FEEDBACK_MS)
    } catch {
      // clipboard permission denied — nothing more we can do here
    }
  }

  return { copiedValue, copy }
}
