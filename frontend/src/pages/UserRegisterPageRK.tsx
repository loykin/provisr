// PoC only (branch poc/resourcekit-forms) — resourcekit-rendered equivalent
// of UserRegisterPage.tsx, kept side by side with the original at /users/new
// for direct comparison. Not wired into real navigation.
import { useMemo } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useQueryClient } from '@tanstack/react-query'
import { DataBodyTemplate } from '@loykin/designkit'
import { createRegistry } from '@loykin/resourcekit'
import type { MutationResolver, Resource } from '@loykin/resourcekit'
import type { KindRenderFn } from '@loykin/resourcekit/react'
import { createDesignKitPlugin } from '@loykin/resourcekit/adapters/designkit'
import { ResourceRenderer } from '@loykin/resourcekit/react'
import { Button } from '@/components/ui/button'
import { createProvisrKindsPlugin } from '@/lib/resourcekit-provisr'
import { createUser } from '@/features/users/api'
import type { CreateUserRequest } from '@/features/users/types'

// v2: ResourceForm + DataBodyGroup/DataBodyRow tree instead of FormView's
// flat sections — DataBodyRow renders through the same designkit
// DataBodyTemplate.Row provisr's own hand-written forms use, so this is the
// test of whether that gets layout parity where FormView (v1) didn't.
function inputRow(label: string, required: boolean | undefined, inputSpec: Record<string, unknown>): Resource {
  return {
    apiVersion: 'resourcekit.dev/v1alpha1',
    kind: 'DataBodyRow',
    spec: { label, required },
    slots: [
      {
        items: [
          { apiVersion: 'resourcekit.dev/v1alpha1', kind: 'InputControl', spec: inputSpec },
        ],
      },
    ],
  }
}

const resource: Resource = {
  apiVersion: 'resourcekit.dev/v1alpha1',
  kind: 'ResourceForm',
  spec: {
    submit: {
      mutation: { target: 'provisr-create-user' },
      onSuccess: [{ kind: 'emit', event: 'provisr.user.created' }],
    },
    submitLabel: 'Create',
    successMessage: 'User created',
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
                inputRow('Username', true, { name: 'username' }),
                inputRow('Password', true, { name: 'password', type: 'password' }),
                inputRow('Email', false, { name: 'email', type: 'email', placeholder: '(optional)' }),
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
}

export default function UserRegisterPageRK() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const registry = useMemo(() => {
    const createUserMutationResolver: MutationResolver = async (_binding, payload) => {
      const body = payload as Record<string, unknown>
      const roles = Array.isArray(body.roles) ? body.roles.map(String) : body.roles ? [String(body.roles)] : []
      const req: CreateUserRequest = {
        username: String(body.username ?? '').trim(),
        password: String(body.password ?? ''),
        email: body.email ? String(body.email).trim() || undefined : undefined,
        roles,
      }
      const user = await createUser(req)
      await queryClient.invalidateQueries({ queryKey: ['users'] })
      return user
    }

    const reg = createRegistry<KindRenderFn>()
    reg.use(createDesignKitPlugin())
    reg.use(createProvisrKindsPlugin())
    reg.use({
      name: 'provisr-mutations',
      mutationResolvers: { 'provisr-create-user': createUserMutationResolver },
    })
    return reg
  }, [queryClient])

  return (
    <div className="flex h-full flex-col">
      <DataBodyTemplate
        title="Create user (resourcekit PoC)"
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
            if (event === 'provisr.user.created') void navigate({ to: '/users' })
          }}
          renderError={(error) => <p className="px-4 text-sm text-destructive">{String(error)}</p>}
        />
      </DataBodyTemplate>
    </div>
  )
}
