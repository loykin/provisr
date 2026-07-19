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
  groups?: string[]
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
  log?: {
    slog?: {
      level?: 'debug' | 'info' | 'warn' | 'error'
      format?: 'text' | 'json'
      color?: boolean
      timestamps?: boolean
      source?: boolean
    }
    file?: {
      dir?: string
      stdoutPath?: string
      stderrPath?: string
      maxSizeMB?: number
      maxBackups?: number
      maxAgeDays?: number
      compress?: boolean
    }
  }
  lifecycle?: LifecycleHooks
}

export interface ProcessMetrics {
  pid: number
  name: string
  cpu_percent: number
  memory_mb: number
  memory_rss: number
  memory_vms: number
  memory_swap?: number
  timestamp: string
  num_threads: number
  num_fds?: number
}

export interface ProcessMetricsHistory {
  process: string
  history: ProcessMetrics[]
}

export interface DebugProcessInfo {
  status: ProcessStatus
  internal_state: string
  health_check: string
}
