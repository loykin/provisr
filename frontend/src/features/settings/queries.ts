import { useQuery } from '@tanstack/react-query'
import { getRuntimeStatus } from './api'

export function useRuntimeStatus() {
  return useQuery({ queryKey: ['runtimeStatus'], queryFn: getRuntimeStatus })
}
