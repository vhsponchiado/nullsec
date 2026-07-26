import { Button } from '@/components/ui/button'

/** Wraps the current text selection of `el` in §…§ (Burp-style payload
 * position markers), or inserts an empty §§ pair at the cursor if nothing's
 * selected. Restores focus and places the cursor right after the closing §
 * so the user can keep typing/marking without losing their place. */
export function wrapWithMarker(
  el: HTMLInputElement | HTMLTextAreaElement,
  value: string,
  onChange: (next: string) => void,
) {
  const start = el.selectionStart ?? value.length
  const end = el.selectionEnd ?? value.length
  const selected = value.slice(start, end)
  const next = value.slice(0, start) + '§' + selected + '§' + value.slice(end)
  onChange(next)
  const cursor = end + 2
  requestAnimationFrame(() => {
    el.focus()
    el.setSelectionRange(cursor, cursor)
  })
}

interface MarkButtonProps {
  getEl: () => HTMLInputElement | HTMLTextAreaElement | null
  value: string
  onChange: (next: string) => void
  className?: string
  // Match whatever height the paired field renders at: "icon" (h-8) next to
  // a default Input, "icon-lg" (h-11) next to a size="lg" Input. Textareas
  // don't need this — the flex row already stretches the button to match.
  size?: 'icon' | 'icon-lg'
}

/** Small "§" toolbar button that marks the current selection of a paired
 * input/textarea as an Intruder payload position. */
export function MarkButton({ getEl, value, onChange, className, size = 'icon' }: MarkButtonProps) {
  return (
    <Button
      type="button"
      size={size}
      variant="outline"
      className={className}
      title="Mark selection as a payload position (§…§)"
      aria-label="Mark selection as a payload position"
      onClick={() => {
        const el = getEl()
        if (!el) return
        wrapWithMarker(el, value, onChange)
      }}
    >
      §
    </Button>
  )
}
