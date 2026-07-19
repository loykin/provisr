import { Navigate } from '@tanstack/react-router'
import { DataPage, PageBreadcrumb } from '@loykin/designkit'
import { DetailList, DetailRow, DetailSection } from '@/components/panel-detail'
import { StatusBadge } from '@/components/status-badge'
import { useAuth } from '@/features/auth/context'
import { useRuntimeStatus } from '@/features/settings/queries'

function Capability({ enabled }: { enabled: boolean }) {
  return <StatusBadge status={enabled ? 'active' : 'disabled'} />
}

export default function SettingsPage() {
  const { user } = useAuth()
  const { data, error } = useRuntimeStatus()
  if (!user?.roles.includes('admin')) return <Navigate to="/processes" replace />

  return (
    <div className="flex h-full flex-col">
      <DataPage.Header>
        <DataPage.TitleBlock title="Settings" breadcrumb={<PageBreadcrumb items={['provisr', 'Settings']} />} />
      </DataPage.Header>
      <div className="min-h-0 flex-1 overflow-y-auto p-4">
        <div className="max-w-2xl space-y-8 rounded-(--radius) border border-border p-4">
          <DetailSection title="Runtime capabilities">
            {error && <p className="text-sm text-destructive">Failed to load runtime settings.</p>}
            {!data && !error && <p className="text-sm text-muted-foreground">Loading…</p>}
            {data && (
              <DetailList>
                <DetailRow label="Authentication"><Capability enabled={data.auth_enabled} /></DetailRow>
                <DetailRow label="Process metrics"><Capability enabled={data.metrics_enabled} /></DetailRow>
                <DetailRow label="History"><Capability enabled={data.history_enabled} /></DetailRow>
                <DetailRow label="Cron scheduler"><Capability enabled={data.cron_scheduler_enabled} /></DetailRow>
                <DetailRow label="Program persistence"><Capability enabled={data.program_persistence} /></DetailRow>
                <DetailRow label="Configured groups">{data.configured_group_count}</DetailRow>
              </DetailList>
            )}
          </DetailSection>
          <p className="text-xs text-muted-foreground">This page is read-only. Sensitive values such as credentials, tokens, DSNs, and environment variables are never returned by the API.</p>
        </div>
      </div>
    </div>
  )
}
