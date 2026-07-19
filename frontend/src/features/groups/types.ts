import type { ProcessStatus } from '@/features/processes/types'

export interface GroupMember {
  name: string
  instances: number
}

export interface GroupInfo {
  name: string
  members: GroupMember[]
  state: 'running' | 'degraded' | 'stopped'
  running: number
  total: number
}

export type GroupStatus = Record<string, ProcessStatus[]>
