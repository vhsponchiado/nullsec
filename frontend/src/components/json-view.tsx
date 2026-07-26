function escapeHtml(str: string): string {
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
}

// Tokenizes already-escaped JSON text and wraps tokens in colored spans.
// Never called with un-escaped input — see JsonView below.
function highlightEscapedJSON(escaped: string): string {
  return escaped.replace(
    /("(?:\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(?:true|false)\b|\bnull\b|-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)/g,
    (match) => {
      let cls = 'text-sky-400' // number
      if (/^&quot;|^"/.test(match)) {
        cls = /:$/.test(match) ? 'text-primary' : 'text-success'
      } else if (/true|false/.test(match)) {
        cls = 'text-warning'
      } else if (/null/.test(match)) {
        cls = 'text-muted-foreground'
      }
      return `<span class="${cls}">${match}</span>`
    },
  )
}

export function tryPrettyJSON(content: string): string | null {
  const trimmed = content.trim()
  if (!trimmed || (trimmed[0] !== '{' && trimmed[0] !== '[')) return null
  try {
    return JSON.stringify(JSON.parse(trimmed), null, 2)
  } catch {
    return null
  }
}

interface JsonViewProps {
  content: string
  emptyLabel?: string
  className?: string
}

/** Renders `content` as pretty-printed, syntax-highlighted JSON when it
 * parses as JSON; otherwise falls back to plain (safely escaped) text. */
export function JsonView({ content, emptyLabel = '(empty)', className }: JsonViewProps) {
  if (!content) {
    return <div className={`text-xs text-muted-foreground p-2 ${className ?? ''}`}>{emptyLabel}</div>
  }

  const pretty = tryPrettyJSON(content)
  if (pretty !== null) {
    const html = highlightEscapedJSON(escapeHtml(pretty))
    return (
      <pre
        className={`font-mono text-xs whitespace-pre-wrap break-all p-2 ${className ?? ''}`}
        dangerouslySetInnerHTML={{ __html: html }}
      />
    )
  }

  return (
    <pre className={`font-mono text-xs whitespace-pre-wrap break-all p-2 ${className ?? ''}`}>
      {content}
    </pre>
  )
}
