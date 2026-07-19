export interface RuntimeStatus {
  auth_enabled: boolean
  metrics_enabled: boolean
  history_enabled: boolean
  cron_scheduler_enabled: boolean
  program_persistence: boolean
  configured_group_count: number
}
