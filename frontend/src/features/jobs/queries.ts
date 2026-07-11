import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createJob, deleteJob, getJob, listJobs, updateJob } from './api'
import type { JobSpec } from './types'

const JOBS_POLL_MS = 3000

export function useJobs() {
  return useQuery({
    queryKey: ['jobs'],
    queryFn: listJobs,
    refetchInterval: JOBS_POLL_MS,
  })
}

export function useJob(name: string, enabled = true) {
  return useQuery({
    queryKey: ['job', name],
    queryFn: () => getJob(name),
    refetchInterval: JOBS_POLL_MS,
    enabled,
  })
}

function useInvalidateJob() {
  const queryClient = useQueryClient()
  return (name?: string) => {
    void queryClient.invalidateQueries({ queryKey: ['jobs'] })
    if (name) void queryClient.invalidateQueries({ queryKey: ['job', name] })
  }
}

export function useCreateJob() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (spec: JobSpec) => createJob(spec),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['jobs'] }),
  })
}

export function useUpdateJob() {
  const invalidate = useInvalidateJob()
  return useMutation({
    mutationFn: (spec: JobSpec) => updateJob(spec),
    onSuccess: (_data, spec) => invalidate(spec.name),
  })
}

export function useDeleteJob() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (name: string) => deleteJob(name),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['jobs'] }),
  })
}
