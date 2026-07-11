import type { ReactNode } from 'react'

export function DetailSection({ title, children }: { title: ReactNode; children: ReactNode }) {
  return (
    <section className="space-y-3">
      <p className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
        {title}
      </p>
      {children}
    </section>
  )
}

export function DetailList({ children }: { children: ReactNode }) {
  return <div className="space-y-2.5">{children}</div>
}

export function DetailRow({ label, children }: { label: ReactNode; children: ReactNode }) {
  return (
    <div className="grid grid-cols-[9rem_minmax(0,1fr)] items-start gap-3">
      <div className="pt-0.5 text-xs text-muted-foreground">{label}</div>
      <div className="min-w-0 break-words text-sm text-foreground">{children}</div>
    </div>
  )
}

export function MonoValue({ children }: { children: ReactNode }) {
  // Monospace glyphs render ~5% taller than sans-serif at the same
  // font-size (measured via canvas.measureText), so this is sized down
  // to optically match the text-sm sans values in sibling DetailRows.
  return <span className="font-mono text-[0.8125rem] leading-5">{children}</span>
}
