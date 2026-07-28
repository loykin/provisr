import type { ReactNode } from 'react'

export function StackedFormField({
  label,
  description,
  required,
  children,
}: {
  label: string
  description?: string
  required?: boolean
  children: ReactNode
}) {
  return (
    <div className="space-y-1.5">
      <div className="flex items-center gap-1">
        <span className="text-xs font-medium">{label}</span>
        {required && <span className="text-xs leading-none text-destructive">*</span>}
      </div>
      {children}
      {description && <p className="text-xs text-muted-foreground">{description}</p>}
    </div>
  )
}
