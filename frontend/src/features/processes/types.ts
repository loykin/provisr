import type { LifecycleHooks } from '@/components/lifecycle-hooks'

export interface ProcessStatus {
  name: string
  running: boolean
  pid: number
  started_at: string
  stopped_at: string
  detected_by: string
  restarts: number
  state: string
}

export interface LogLine {
  offset: number
  stream: 'stdout' | 'stderr'
  text: string
}

export interface LogsSinceResponse {
  lines: LogLine[]
  next: number
}

export interface ProcessSpec {
  name: string
  command?: string
  args?: string[]
  work_dir?: string
  env?: string[]
  pid_file?: string
  priority?: number
  retry_count?: number
  retry_interval?: string | number
  start_duration?: string | number
  auto_restart?: boolean
  restart_interval?: string | number
  instances?: number
  detached?: boolean
  detectors?: Array<Record<string, unknown>>
  log?: Record<string, unknown>
  lifecycle?: LifecycleHooks
}
