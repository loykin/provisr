import type { LifecycleHooks } from '@/components/lifecycle-hooks'

export interface JobTemplate {
  name?: string
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

export interface CronJobSpec {
  name: string
  schedule: string
  job_template: JobTemplate
  concurrency_policy?: 'Allow' | 'Forbid' | 'Replace' | ''
  suspend?: boolean | null
  successful_jobs_history_limit?: number
  failed_jobs_history_limit?: number
  starting_deadline_seconds?: number
  time_zone?: string
  lifecycle?: LifecycleHooks
}

export interface JobReference {
  name: string
}

export interface CronJobStatus {
  active?: JobReference[]
  last_schedule_time?: string
  last_successful_time?: string
}

// GET /cronjobs and /cronjobs/:name response: CronJobSpec flattened plus status/next_schedule.
export interface CronJobInfo extends CronJobSpec {
  status: CronJobStatus
  next_schedule?: string
}

export interface CronJobHistoryEntry {
  Name: string
  StartTime: string
  CompletionTime?: string
  Status: 'Pending' | 'Running' | 'Succeeded' | 'Failed'
  Reason: string
}
