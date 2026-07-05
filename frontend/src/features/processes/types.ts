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
