import { X } from 'lucide-react'
import { PanelTemplate } from '@loykin/designkit'
import { useSidePanel } from '@loykin/side-panel'
import { Button } from '@/components/ui/button'
import { StatusBadge } from '@/components/status-badge'
import { DetailList, DetailRow, DetailSection } from '@/components/panel-detail'
import { ProcessStateBadge } from '@/features/processes/ProcessStateBadge'
import { GroupActions } from './GroupActions'
import { useGroups, useGroupStatus } from './queries'
import type { GroupInfo } from './types'

export function GroupDetailPanel({ group }: { group: GroupInfo }) {
  const { close } = useSidePanel()
  const { data: status, error } = useGroupStatus(group.name)
  const { data: groups } = useGroups()
  const currentGroup = groups?.find((item) => item.name === group.name) ?? group
  const closeButton = (
    <Button variant="ghost" size="icon-sm" onClick={() => void close()}>
      <X className="h-3.5 w-3.5" />
    </Button>
  )

  return (
    <PanelTemplate
      eyebrow="Group"
      title={group.name}
      status={<StatusBadge status={currentGroup.state} />}
      actions={<div className="flex items-center gap-2"><GroupActions group={currentGroup} />{closeButton}</div>}
    >
      <DetailSection title="Summary">
        <DetailList>
          <DetailRow label="Members">{group.members.length}</DetailRow>
          <DetailRow label="Running">{currentGroup.running} / {currentGroup.total}</DetailRow>
        </DetailList>
      </DetailSection>
      <DetailSection title="Members">
        {error && <p className="text-sm text-destructive">Failed to load group status.</p>}
        {!status && !error && <p className="text-sm text-muted-foreground">Loading…</p>}
        <div className="overflow-hidden rounded-(--radius) border border-border">
          <table className="w-full text-left text-sm">
            <thead className="bg-muted/50 text-xs text-muted-foreground">
              <tr>
                <th className="px-3 py-2 font-medium">Member</th>
                <th className="px-3 py-2 font-medium">Instance</th>
                <th className="px-3 py-2 font-medium">State</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {currentGroup.members.flatMap((member) => {
                const instances = status?.[member.name] ?? []
                if (instances.length === 0) {
                  return [(
                    <tr key={member.name}>
                      <td className="px-3 py-2 font-medium">{member.name}</td>
                      <td className="px-3 py-2 text-muted-foreground">No registered instance</td>
                      <td className="px-3 py-2 text-muted-foreground">-</td>
                    </tr>
                  )]
                }
                return instances.map((instance, index) => (
                  <tr key={instance.name}>
                    <td className="px-3 py-2 font-medium">{index === 0 ? member.name : ''}</td>
                    <td className="px-3 py-2 font-mono text-xs">{instance.name}</td>
                    <td className="px-3 py-2"><ProcessStateBadge state={instance.state} /></td>
                  </tr>
                ))
              })}
            </tbody>
          </table>
        </div>
      </DetailSection>
    </PanelTemplate>
  )
}
