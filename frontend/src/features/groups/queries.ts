import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { getGroupStatus, listGroups, startGroup, stopGroup } from './api'

const GROUP_POLL_MS = 3000

export function useGroups() {
  return useQuery({ queryKey: ['groups'], queryFn: listGroups, refetchInterval: GROUP_POLL_MS })
}

export function useGroupStatus(name: string, enabled = true) {
  return useQuery({
    queryKey: ['group', name],
    queryFn: () => getGroupStatus(name),
    enabled: enabled && Boolean(name),
    refetchInterval: GROUP_POLL_MS,
  })
}

function useGroupMutation(action: (name: string) => Promise<void>) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: action,
    onSuccess: (_data, name) => {
      void queryClient.invalidateQueries({ queryKey: ['groups'] })
      void queryClient.invalidateQueries({ queryKey: ['group', name] })
      void queryClient.invalidateQueries({ queryKey: ['processes'] })
    },
  })
}

export function useStartGroup() {
  return useGroupMutation(startGroup)
}

export function useStopGroup() {
  return useGroupMutation(stopGroup)
}
