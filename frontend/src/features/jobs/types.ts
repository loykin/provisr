import type { LifecycleHooks } from '@/components/lifecycle-hooks'

export interface JobSpec {
  name: string
  command?: string
  args?: string[]
  work_dir?: string
  env?: string[]
  ttl_seconds_after_finished?: number
  active_deadline_seconds?: number
  backoff_limit?: number
  parallelism?: number
  completions?: number
  completion_mode?: 'NonIndexed' | 'Indexed' | ''
  restart_policy?: 'Never' | 'OnFailure' | ''
  lifecycle?: LifecycleHooks
  depends_on?: string[]
}

export interface JobStatus {
  phase: 'Pending' | 'Running' | 'Succeeded' | 'Failed'
  start_time?: string
  completion_time?: string
  active: number
  succeeded: number
  failed: number
}

export interface JobInfo extends JobSpec {
  status: JobStatus
}
