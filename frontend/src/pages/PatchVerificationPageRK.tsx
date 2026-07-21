// PoC only (branch poc/resourcekit-forms) — verifies the resourcekit patch
// (commit f847fb8) actually closes the gaps found in provisr-poc-findings.md
// without provisr's custom-kind workarounds:
// - #1/#8: InputControl required/disabled now enforced by the browser.
// - #2: ResourceForm.spec.id + hideSubmitButton let the submit button live
//   in the page header instead of the form body.
// - #3/#4/#5: first-party Textarea/Checkbox/Select kinds replace
//   TextareaField/CheckboxGroup/Select from resourcekit-provisr.tsx.
// - #10: visible now supports $or directly — no derived `canWrite`
//   variable needed for the admin-OR-operator pattern.
import { useMemo, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { createRegistry } from '@loykin/resourcekit'
import type { MutationResolver, Resource } from '@loykin/resourcekit'
import type { KindRenderFn } from '@loykin/resourcekit/react'
import { createDesignKitPlugin } from '@loykin/resourcekit/adapters/designkit'
import { ResourceRenderer } from '@loykin/resourcekit/react'
import { DataBodyTemplate } from '@loykin/designkit'
import { Button } from '@/components/ui/button'
import { useAuth } from '@/features/auth/context'

const FORM_ID = 'patch-verification-form'

function buildResource(roles: string[]): Resource {
  return {
    apiVersion: 'resourcekit.dev/v1alpha1',
    kind: 'ResourceForm',
    spec: {
      id: FORM_ID,
      hideSubmitButton: true,
      submit: { mutation: { target: 'echo' } },
    },
    slots: [
      {
        items: [
          {
            apiVersion: 'resourcekit.dev/v1alpha1',
            kind: 'DataBodyGroup',
            spec: {
              layout: 'stacked',
              variables: [{ name: 'roles', type: 'string[]', default: roles }],
            },
            slots: [
              {
                items: [
                  {
                    apiVersion: 'resourcekit.dev/v1alpha1',
                    kind: 'DataBodyRow',
                    spec: { label: 'Name (required)', required: true },
                    slots: [{ items: [{ apiVersion: 'resourcekit.dev/v1alpha1', kind: 'InputControl', spec: { name: 'name', required: true } }] }],
                  },
                  {
                    apiVersion: 'resourcekit.dev/v1alpha1',
                    kind: 'DataBodyRow',
                    spec: { label: 'Environment (Textarea kind)' },
                    slots: [{ items: [{ apiVersion: 'resourcekit.dev/v1alpha1', kind: 'Textarea', spec: { name: 'env', rows: 3 } }] }],
                  },
                  {
                    apiVersion: 'resourcekit.dev/v1alpha1',
                    kind: 'DataBodyRow',
                    spec: { label: 'Concurrency policy (Select kind)' },
                    slots: [
                      {
                        items: [
                          {
                            apiVersion: 'resourcekit.dev/v1alpha1',
                            kind: 'Select',
                            spec: {
                              name: 'concurrencyPolicy',
                              options: [
                                { label: 'Allow', value: 'Allow' },
                                { label: 'Forbid', value: 'Forbid' },
                              ],
                            },
                          },
                        ],
                      },
                    ],
                  },
                  {
                    apiVersion: 'resourcekit.dev/v1alpha1',
                    kind: 'DataBodyRow',
                    spec: { label: 'Roles (Checkbox kind)' },
                    slots: [
                      {
                        items: [
                          { apiVersion: 'resourcekit.dev/v1alpha1', kind: 'Checkbox', spec: { name: 'roles', label: 'admin', value: 'admin' } },
                          { apiVersion: 'resourcekit.dev/v1alpha1', kind: 'Checkbox', spec: { name: 'roles', label: 'operator', value: 'operator' } },
                          { apiVersion: 'resourcekit.dev/v1alpha1', kind: 'Checkbox', spec: { name: 'roles', label: 'viewer', value: 'viewer' } },
                        ],
                      },
                    ],
                  },
                  {
                    apiVersion: 'resourcekit.dev/v1alpha1',
                    kind: 'DataBodyField',
                    spec: { label: 'Write access (admin OR operator, via $or — no derived variable)', value: 'visible via native $or' },
                    visible: { $or: [{ $variable: 'roles', contains: 'admin' }, { $variable: 'roles', contains: 'operator' }] },
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

export default function PatchVerificationPageRK() {
  const navigate = useNavigate()
  const { user } = useAuth()
  const roles = useMemo(() => user?.roles ?? [], [user])
  const [result, setResult] = useState<unknown>(null)

  const resource = useMemo(() => buildResource(roles), [roles])

  const registry = useMemo(() => {
    const echoResolver: MutationResolver = async (_binding, payload) => {
      setResult(payload)
      return payload
    }
    const reg = createRegistry<KindRenderFn>()
    reg.use(createDesignKitPlugin())
    reg.use({ name: 'demo-mutations', mutationResolvers: { echo: echoResolver } })
    return reg
  }, [])

  return (
    <div className="flex h-full flex-col">
      <DataBodyTemplate
        title={`Patch verification (resourcekit PoC) — ${user?.username ?? '?'} [${roles.join(', ')}]`}
        contentClassName="flex-1"
        actions={
          <>
            <Button type="button" variant="ghost" onClick={() => void navigate({ to: '/processes' })}>
              Cancel
            </Button>
            <Button type="submit" form={FORM_ID}>
              Save (header button via form=id)
            </Button>
          </>
        }
      >
        <div className="p-4">
          <ResourceRenderer registry={registry} resource={resource} />
          {result != null && (
            <div className="mt-6">
              <p className="mb-1 text-sm font-medium">Submitted payload:</p>
              <pre className="rounded-(--radius) border border-border bg-muted p-3 text-xs">{JSON.stringify(result, null, 2)}</pre>
            </div>
          )}
        </div>
      </DataBodyTemplate>
    </div>
  )
}
