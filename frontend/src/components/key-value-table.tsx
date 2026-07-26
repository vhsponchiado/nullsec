import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

interface KeyValueTableProps {
  pairs: Array<[string, string]>
  emptyLabel: string
}

export function KeyValueTable({ pairs, emptyLabel }: KeyValueTableProps) {
  if (pairs.length === 0) {
    return <div className="text-sm text-muted-foreground py-6 text-center">{emptyLabel}</div>
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Key</TableHead>
          <TableHead>Value</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {pairs.map(([key, value], i) => (
          <TableRow key={`${key}-${i}`}>
            <TableCell className="font-mono text-xs align-top whitespace-nowrap">{key}</TableCell>
            <TableCell className="font-mono text-xs break-all">{value}</TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

export function headersToPairs(headers: Record<string, string[]> | undefined): Array<[string, string]> {
  if (!headers) return []
  const pairs: Array<[string, string]> = []
  for (const [key, values] of Object.entries(headers)) {
    for (const value of values) pairs.push([key, value])
  }
  return pairs
}

export function queryParamsToPairs(url: string): Array<[string, string]> {
  try {
    const u = new URL(url)
    return [...u.searchParams.entries()]
  } catch {
    return []
  }
}
