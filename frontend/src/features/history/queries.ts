import { useQuery } from '@tanstack/react-query'
import { getHistory } from './api'

const HISTORY_POLL_MS = 5000

export function useHistory(name?: string) {
  return useQuery({
    queryKey: ['history', name ?? ''],
    queryFn: () => getHistory(name),
    refetchInterval: HISTORY_POLL_MS,
  })
}
