// PoC only (branch poc/resourcekit-forms) — verifies finding #10: does
// resourcekit's `visible` + variable mechanism actually let a host inject
// the logged-in user's roles and gate content by them? `ResourceRenderer`
// builds its own VariableEngine per mount and seeds it via
// `collectVariables()`, which walks the resource tree pulling any kind's
// `spec.variables` array — so the injection point is "put a `variables`
// declaration with a `default` in some node's spec," not a dedicated prop.
// `DataBodyGroup`'s specSchema is `additionalProperties: true`, so this
// doesn't even violate its schema.
import { useMemo } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { createRegistry } from '@loykin/resourcekit'
import type { Resource } from '@loykin/resourcekit'
import type { KindRenderFn } from '@loykin/resourcekit/react'
import { createDesignKitPlugin } from '@loykin/resourcekit/adapters/designkit'
import { ResourceRenderer } from '@loykin/resourcekit/react'
import { DataBodyTemplate } from '@loykin/designkit'
import { Button } from '@/components/ui/button'
import { useAuth } from '@/features/auth/context'
import { canWriteWorkloads } from '@/features/auth/permissions'

function buildResource(roles: string[], canWrite: boolean): Resource {
  return {
    apiVersion: 'resourcekit.dev/v1alpha1',
    kind: 'DataBodyGroup',
    spec: {
      layout: 'stacked',
      // The injection point: a per-render-computed `default`, sourced from
      // provisr's own AuthProvider, not from the document itself.
      variables: [
        { name: 'roles', type: 'string[]', default: roles },
        { name: 'canWrite', type: 'string', default: canWrite ? 'true' : '' },
      ],
    },
    slots: [
      {
        items: [
          {
            apiVersion: 'resourcekit.dev/v1alpha1',
            kind: 'DataBodyField',
            spec: { label: 'Always visible', value: 'everyone sees this row' },
          },
          {
            apiVersion: 'resourcekit.dev/v1alpha1',
            kind: 'DataBodyField',
            spec: { label: 'Admin only', value: 'visible: {$variable: roles, contains: admin}' },
            visible: { $variable: 'roles', contains: 'admin' },
          },
          {
            apiVersion: 'resourcekit.dev/v1alpha1',
            kind: 'DataBodyField',
            spec: { label: 'Write access (admin OR operator)', value: 'visible: {$variable: canWrite} — derived-variable OR workaround' },
            visible: { $variable: 'canWrite' },
          },
        ],
      },
    ],
  }
}

export default function PermissionsDemoPageRK() {
  const navigate = useNavigate()
  const { user } = useAuth()
  const roles = useMemo(() => user?.roles ?? [], [user])
  const canWrite = canWriteWorkloads(user)

  const resource = useMemo(() => buildResource(roles, canWrite), [roles, canWrite])

  const registry = useMemo(() => {
    const reg = createRegistry<KindRenderFn>()
    reg.use(createDesignKitPlugin())
    return reg
  }, [])

  return (
    <div className="flex h-full flex-col">
      <DataBodyTemplate
        title={`Permissions demo (resourcekit PoC) — logged in as ${user?.username ?? '?'} [${roles.join(', ')}]`}
        contentClassName="flex-1"
        actions={
          <Button type="button" variant="ghost" onClick={() => void navigate({ to: '/processes' })}>
            Back
          </Button>
        }
      >
        <ResourceRenderer registry={registry} resource={resource} />
      </DataBodyTemplate>
    </div>
  )
}
