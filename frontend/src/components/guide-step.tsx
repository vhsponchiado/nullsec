import type { ReactNode } from 'react'

interface StepProps {
  n: number
  title: string
  children: ReactNode
}

/** One numbered step in a `<feature>-guide.tsx` explainer panel. */
export function Step({ n, title, children }: StepProps) {
  return (
    <div className="flex gap-3">
      <div className="size-6 rounded-full bg-primary/15 text-primary text-xs font-semibold flex items-center justify-center shrink-0 mt-0.5">
        {n}
      </div>
      <div className="flex flex-col gap-1">
        <p className="text-sm font-medium">{title}</p>
        <p className="text-xs text-muted-foreground">{children}</p>
      </div>
    </div>
  )
}
