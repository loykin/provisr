// Subset of job.Spec the create/edit form exposes (see core/internal/job/spec.go).
export interface JobTemplate {
  name?: string
  command: string
  work_dir?: string
  env?: string[]
}

// Wire shape of core.CronJob (cronjob.CronJobSpec), the fields the register/edit
// form exposes plus concurrency_policy/suspend which the list also shows.
export interface CronJobSpec {
  name: string
  schedule: string
  job_template: JobTemplate
  concurrency_policy?: 'Allow' | 'Forbid' | 'Replace' | ''
  suspend?: boolean | null
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
