import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  createCronJob,
  deleteCronJob,
  getCronJob,
  getCronJobHistory,
  listCronJobs,
  resumeCronJob,
  suspendCronJob,
  triggerCronJob,
  updateCronJob,
} from './api'
import type { CronJobSpec } from './types'

const CRONJOBS_POLL_MS = 5000

export function useCronJobs() {
  return useQuery({
    queryKey: ['cronjobs'],
    queryFn: listCronJobs,
    refetchInterval: CRONJOBS_POLL_MS,
  })
}

export function useCronJob(name: string, enabled = true) {
  return useQuery({
    queryKey: ['cronjob', name],
    queryFn: () => getCronJob(name),
    refetchInterval: CRONJOBS_POLL_MS,
    enabled,
  })
}

export function useCronJobHistory(name: string, enabled = true) {
  return useQuery({
    queryKey: ['cronjobHistory', name],
    queryFn: () => getCronJobHistory(name),
    refetchInterval: CRONJOBS_POLL_MS,
    enabled,
  })
}

function useInvalidateCronJob() {
  const queryClient = useQueryClient()
  return (name: string) => {
    void queryClient.invalidateQueries({ queryKey: ['cronjobs'] })
    void queryClient.invalidateQueries({ queryKey: ['cronjob', name] })
    void queryClient.invalidateQueries({ queryKey: ['cronjobHistory', name] })
  }
}

export function useCreateCronJob() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (spec: CronJobSpec) => createCronJob(spec),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['cronjobs'] }),
  })
}

export function useUpdateCronJob() {
  const invalidate = useInvalidateCronJob()
  return useMutation({
    mutationFn: (spec: CronJobSpec) => updateCronJob(spec),
    onSuccess: (_data, spec) => invalidate(spec.name),
  })
}

export function useDeleteCronJob() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (name: string) => deleteCronJob(name),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['cronjobs'] }),
  })
}

export function useSuspendCronJob() {
  const invalidate = useInvalidateCronJob()
  return useMutation({
    mutationFn: (name: string) => suspendCronJob(name),
    onSuccess: (_data, name) => invalidate(name),
  })
}

export function useResumeCronJob() {
  const invalidate = useInvalidateCronJob()
  return useMutation({
    mutationFn: (name: string) => resumeCronJob(name),
    onSuccess: (_data, name) => invalidate(name),
  })
}

export function useTriggerCronJob() {
  const invalidate = useInvalidateCronJob()
  return useMutation({
    mutationFn: (name: string) => triggerCronJob(name),
    onSuccess: (_data, name) => invalidate(name),
  })
}
