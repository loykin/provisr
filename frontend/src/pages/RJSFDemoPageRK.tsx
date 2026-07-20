// PoC only (branch poc/resourcekit-forms) — standalone mechanism test, not
// wired to provisr's real API. Proves two things the designkit-based custom
// kinds (CheckboxGroup/TextareaField/Select/LifecycleHooksField) couldn't:
// cross-field validation (password confirm-match) and a native array-of-
// objects field (no hidden-JSON-string workaround needed) — both via a
// JSONSchemaForm kind that wraps react-jsonschema-form as an ordinary
// resourcekit adapter.
import { useMemo, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { createRegistry } from '@loykin/resourcekit'
import type { MutationResolver, Resource } from '@loykin/resourcekit'
import type { KindRenderFn } from '@loykin/resourcekit/react'
import { ResourceRenderer } from '@loykin/resourcekit/react'
import { DataBodyTemplate } from '@loykin/designkit'
import { Button } from '@/components/ui/button'
import { createRJSFPlugin } from '@/lib/resourcekit-rjsf'

const resource: Resource = {
  apiVersion: 'provisr.dev/v1alpha1',
  kind: 'JSONSchemaForm',
  spec: {
    jsonSchema: {
      type: 'object',
      required: ['name', 'password', 'confirmPassword'],
      properties: {
        name: { type: 'string', title: 'Name' },
        password: { type: 'string', title: 'Password' },
        confirmPassword: { type: 'string', title: 'Confirm password' },
        hooks: {
          type: 'array',
          title: 'Pre-start hooks',
          items: {
            type: 'object',
            required: ['name', 'command'],
            properties: {
              name: { type: 'string', title: 'Hook name' },
              command: { type: 'string', title: 'Command' },
            },
          },
        },
      },
    },
    uiSchema: {
      password: { 'ui:widget': 'password' },
      confirmPassword: { 'ui:widget': 'password' },
    },
    submit: { mutation: { target: 'demo-echo' } },
    submitLabel: 'Submit',
    customValidateKey: 'passwordsMatch',
  },
}

export default function RJSFDemoPageRK() {
  const navigate = useNavigate()
  const [result, setResult] = useState<unknown>(null)

  const registry = useMemo(() => {
    const echoResolver: MutationResolver = async (_binding, payload) => {
      setResult(payload)
      return payload
    }

    const reg = createRegistry<KindRenderFn>()
    reg.use(
      createRJSFPlugin({
        passwordsMatch: (formData: { password?: string; confirmPassword?: string }, errors) => {
          if (formData.password !== formData.confirmPassword) {
            errors.confirmPassword?.addError('Passwords must match')
          }
          return errors
        },
      }),
    )
    reg.use({ name: 'demo-mutations', mutationResolvers: { 'demo-echo': echoResolver } })
    return reg
  }, [])

  return (
    <div className="flex h-full flex-col">
      <DataBodyTemplate
        title="RJSF adapter mechanism test (resourcekit PoC)"
        contentClassName="flex-1"
        actions={
          <Button type="button" variant="ghost" onClick={() => void navigate({ to: '/processes' })}>
            Back
          </Button>
        }
      >
        <div className="p-4">
          <ResourceRenderer registry={registry} resource={resource} />
          {result != null && (
            <div className="mt-6">
              <p className="mb-1 text-sm font-medium">Submitted payload:</p>
              <pre className="rounded-(--radius) border border-border bg-muted p-3 text-xs">
                {JSON.stringify(result, null, 2)}
              </pre>
            </div>
          )}
        </div>
      </DataBodyTemplate>
    </div>
  )
}
