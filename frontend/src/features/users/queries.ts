import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createUser, deleteUser, listUsers, updateUser } from './api'
import type { CreateUserRequest, UpdateUserRequest } from './types'

export function useUsers() {
  return useQuery({ queryKey: ['users'], queryFn: listUsers })
}

export function useCreateUser() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (req: CreateUserRequest) => createUser(req),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['users'] })
    },
  })
}

export function useUpdateUser() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, req }: { id: string; req: UpdateUserRequest }) => updateUser(id, req),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['users'] })
    },
  })
}

export function useDeleteUser() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => deleteUser(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['users'] })
    },
  })
}
