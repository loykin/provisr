import { PageBreadcrumb, PageTopBar } from '@loykin/designkit'

export function PageBreadcrumbTopBar({ items }: { items: string[] }) {
  return <PageTopBar left={<PageBreadcrumb items={items} />} />
}
