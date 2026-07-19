import type { ReactNode } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { DataPage, PageBreadcrumb } from '@loykin/designkit'

type WorkloadTabId = 'processes' | 'jobs' | 'cronjobs' | 'groups'

const tabs: Array<{ id: WorkloadTabId; label: string; to: '/processes' | '/jobs' | '/cronjobs' | '/groups' }> = [
  { id: 'processes', label: 'Processes', to: '/processes' },
  { id: 'jobs', label: 'Jobs', to: '/jobs' },
  { id: 'cronjobs', label: 'CronJobs', to: '/cronjobs' },
  { id: 'groups', label: 'Groups', to: '/groups' },
]

export function WorkloadHeader({
  active,
  title,
  actions,
}: {
  active: WorkloadTabId
  title: string
  actions?: ReactNode
}) {
  const navigate = useNavigate()

  return (
    <>
      <DataPage.Header>
        <DataPage.TitleBlock
          title={title}
          breadcrumb={<PageBreadcrumb items={['provisr', 'Workloads', title]} />}
        />
        <DataPage.Actions>{actions}</DataPage.Actions>
      </DataPage.Header>
      <DataPage.Tabs>
        {tabs.map((tab) => {
          const to = tab.to
          return (
            <DataPage.Tab
              key={tab.id}
              active={active === tab.id}
              onClick={() => void navigate({ to })}
            >
              {tab.label}
            </DataPage.Tab>
          )
        })}
      </DataPage.Tabs>
    </>
  )
}
