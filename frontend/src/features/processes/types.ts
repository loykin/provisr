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
