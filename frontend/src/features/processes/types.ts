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

// Subset of core.Spec that the register/edit form exposes. The backend
// accepts (and edit-fetches) the full Spec shape, but v1 only surfaces the
// fields an operator would plausibly want to set by hand.
export interface ProcessSpec {
  name: string
  command: string
  work_dir?: string
  env?: string[]
  auto_restart?: boolean
  instances?: number
}
