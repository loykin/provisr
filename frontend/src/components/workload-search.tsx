import { FilterInput } from '@loykin/filter-input'
import { Search } from 'lucide-react'

export function WorkloadSearch({
  value,
  onChange,
  placeholder = 'Search…',
}: {
  value: string
  onChange: (value: string) => void
  placeholder?: string
}) {
  return (
    <div role="search" aria-label="Search table" className="workload-search relative w-56 shrink-0">
      <Search className="pointer-events-none absolute left-2.5 top-1/2 z-10 size-3.5 -translate-y-1/2 text-muted-foreground" />
      <FilterInput
        config={{
          key: 'global-search',
          type: 'text',
          placeholder,
        }}
        value={value}
        onChange={(next) => onChange(typeof next === 'string' ? next : '')}
        inputClassName="pl-8"
      />
    </div>
  )
}
