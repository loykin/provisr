import { useMutation, useQuery } from '@tanstack/react-query'
import { listTemplateTypes, previewTemplate } from './api'

export function useTemplateTypes() {
  return useQuery({ queryKey: ['templateTypes'], queryFn: listTemplateTypes })
}

export function useTemplatePreview() {
  return useMutation({ mutationFn: ({ kind, name }: { kind: string; name: string }) => previewTemplate(kind, name) })
}
