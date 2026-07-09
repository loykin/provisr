import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { getHistory } from './api'

const HISTORY_POLL_MS = 5000

export function useHistory(name: string | undefined, pageIndex: number, pageSize: number) {
  return useQuery({
    queryKey: ['history', name ?? '', pageIndex, pageSize],
    queryFn: () => getHistory(name, pageSize, pageIndex * pageSize),
    placeholderData: keepPreviousData,
    refetchInterval: HISTORY_POLL_MS,
  })
}
