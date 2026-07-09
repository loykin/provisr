export interface HistoryRecord {
  timestamp: string
  pid: number
  name: string
  status: string
  error?: string
}

export interface HistoryPage {
  rows: HistoryRecord[]
  total: number
}
