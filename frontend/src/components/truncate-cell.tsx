import type { ReactNode } from 'react'

// gridkit's default cell box doesn't force `white-space: nowrap` — only
// `meta.wrap: true` opts into wrapping. Plain text/date cells need this
// span (Tailwind's `truncate`) to actually get single-line + ellipsis
// instead of wrapping to multiple rows at the browser's default white-space.
export function TruncateCell({ children }: { children: ReactNode }) {
  return <span className="block w-full truncate">{children}</span>
}
