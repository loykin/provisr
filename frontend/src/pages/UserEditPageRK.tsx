// PoC only (branch poc/resourcekit-forms) — resourcekit-rendered equivalent
// of UserEditPage.tsx. Exercises two things UserRegisterPageRK didn't yet:
// authenticated data read (RecordScope fetching the existing user) and
// fieldRef prefill for edit mode. Password change is intentionally left out
// — that field's cross-field confirm-match validation doesn't fit a
// declarative kind and is a separate finding, not this test's concern.
import { useMemo } from 'react'
import { useNavigate, useParams } from '@tanstack/react-router'
import { useQueryClient } from '@tanstack/react-query'
import { DataBodyTemplate } from '@loykin/designkit'
import { createRegistry } from '@loykin/resourcekit'
import type { MutationResolver, Resource } from '@loykin/resourcekit'
import type { KindRenderFn } from '@loykin/resourcekit/react'
import { createDesignKitPlugin } from '@loykin/resourcekit/adapters/designkit'
import { ResourceRenderer } from '@loykin/resourcekit/react'
import { Button } from '@/components/ui/button'
import { createProvisrKindsPlugin, provisrRestResolver } from '@/lib/resourcekit-provisr'
import { updateUser } from '@/features/users/api'
import type { UpdateUserRequest } from '@/features/users/types'

function inputRow(label: string, fieldRef: string, inputSpec: Record<string, unknown> = {}): Resource {
  return {
    apiVersion: 'resourcekit.dev/v1alpha1',
    kind: 'DataBodyRow',
    spec: { label },
    slots: [
      { items: [{ apiVersion: 'resourcekit.dev/v1alpha1', kind: 'InputControl', spec: { fieldRef, ...inputSpec } }] },
    ],
  }
}

function buildResource(id: string): Resource {
  return {
    apiVersion: 'resourcekit.dev/v1alpha1',
    kind: 'RecordScope',
    spec: { data: { source: 'provisr-rest', path: `/auth/users/${encodeURIComponent(id)}` } },
    slots: [
      {
        items: [
          {
            apiVersion: 'resourcekit.dev/v1alpha1',
            kind: 'ResourceForm',
            spec: {
              submit: {
                mutation: { target: 'provisr-update-user', id },
                onSuccess: [{ kind: 'emit', event: 'provisr.user.updated' }],
              },
              submitLabel: 'Save',
            },
            slots: [
              {
                items: [
                  {
                    apiVersion: 'resourcekit.dev/v1alpha1',
                    kind: 'DataBodyGroup',
                    spec: { layout: 'stacked' },
                    slots: [
                      {
                        items: [
                          inputRow('Username', 'username', { name: 'username' }),
                          inputRow('Email', 'email', { name: 'email', type: 'email', placeholder: '(optional)' }),
                          {
                            apiVersion: 'resourcekit.dev/v1alpha1',
                            kind: 'DataBodyRow',
                            spec: { label: 'Roles' },
                            slots: [
                              {
                                items: [
                                  {
                                    apiVersion: 'provisr.dev/v1alpha1',
                                    kind: 'CheckboxGroup',
                                    spec: {
                                      name: 'roles',
                                      fieldRef: 'roles',
                                      options: [
                                        { label: 'admin', value: 'admin' },
                                        { label: 'operator', value: 'operator' },
                                        { label: 'viewer', value: 'viewer' },
                                      ],
                                    },
                                  },
                                ],
                              },
                            ],
                          },
                        ],
                      },
                    ],
                  },
                ],
              },
            ],
          },
        ],
      },
    ],
  }
}

export default function UserEditPageRK() {
  const { id } = useParams({ strict: false }) as { id: string }
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const resource = useMemo(() => buildResource(id), [id])

  const registry = useMemo(() => {
    const updateUserMutationResolver: MutationResolver = async (binding, payload) => {
      const targetId = (binding as unknown as { id: string }).id
      const body = payload as Record<string, unknown>
      const roles = Array.isArray(body.roles) ? body.roles.map(String) : body.roles ? [String(body.roles)] : []
      const req: UpdateUserRequest = {
        email: body.email ? String(body.email).trim() || undefined : undefined,
        roles,
      }
      const user = await updateUser(targetId, req)
      await queryClient.invalidateQueries({ queryKey: ['users'] })
      return user
    }

    const reg = createRegistry<KindRenderFn>()
    reg.use(createDesignKitPlugin())
    reg.use(createProvisrKindsPlugin())
    reg.use({
      name: 'provisr-mutations',
      mutationResolvers: { 'provisr-update-user': updateUserMutationResolver },
      dataResolvers: { 'provisr-rest': provisrRestResolver },
    })
    return reg
  }, [queryClient])

  return (
    <div className="flex h-full flex-col">
      <DataBodyTemplate
        title="Edit user (resourcekit PoC)"
        contentClassName="flex-1"
        actions={
          <Button type="button" variant="ghost" onClick={() => void navigate({ to: '/users' })}>
            Cancel
          </Button>
        }
      >
        <ResourceRenderer
          registry={registry}
          resource={resource}
          onEvent={(event) => {
            if (event === 'provisr.user.updated') void navigate({ to: '/users' })
          }}
          renderLoading={() => <p className="px-4 text-sm text-muted-foreground">Loading…</p>}
          renderError={(error) => <p className="px-4 text-sm text-destructive">{String(error)}</p>}
        />
      </DataBodyTemplate>
    </div>
  )
}
