import type { ReactNode } from 'react'
import { DataPage, PageBreadcrumb } from '@loykin/designkit'

// A page-level header (breadcrumb + large title + right-aligned actions),
// matching designkit's own DataPage.Header/TitleBlock/Actions composition.
// PageTopBar alone renders its `left` text as a single small breadcrumb
// crumb (text-xs) with no heading — fine for a compact toolbar, but reads
// as "the title is tiny and there's no breadcrumb" when used as a page's
// only header, which is what every list page here was doing.
export function PageHeader({ title, actions }: { title: string; actions?: ReactNode }) {
  return (
    <DataPage.Header>
      <DataPage.TitleBlock title={title} breadcrumb={<PageBreadcrumb items={['provisr', title]} />} />
      <DataPage.Actions>{actions}</DataPage.Actions>
    </DataPage.Header>
  )
}
