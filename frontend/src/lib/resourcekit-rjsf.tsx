// PoC only (branch poc/resourcekit-forms) — proves react-jsonschema-form can
// plug into resourcekit as an ordinary adapter, the same shape as the
// designkit/gridkit adapters (a ResourceKitPlugin registering one kind).
// Targets the two gaps the designkit-based kinds above couldn't close:
// cross-field validation and native array-of-objects fields.
import { useState } from 'react'
import Form from '@rjsf/core'
import validator from '@rjsf/validator-ajv8'
import type { RJSFSchema, UiSchema, CustomValidator } from '@rjsf/utils'
import type { Resource, SubmitSpec } from '@loykin/resourcekit'
import type { RenderContext } from '@loykin/resourcekit/react'

interface JSONSchemaFormSpec {
  jsonSchema: RJSFSchema
  uiSchema?: UiSchema
  submit: SubmitSpec
  submitLabel?: string
  customValidateKey?: string
}

function JSONSchemaFormNode({
  spec,
  ctx,
  customValidators,
}: {
  spec: JSONSchemaFormSpec
  ctx: RenderContext
  customValidators: Record<string, CustomValidator>
}) {
  const [formData, setFormData] = useState<unknown>(undefined)
  const customValidate = spec.customValidateKey ? customValidators[spec.customValidateKey] : undefined
  return (
    <Form
      schema={spec.jsonSchema}
      uiSchema={spec.uiSchema}
      formData={formData}
      validator={validator}
      customValidate={customValidate}
      onChange={(e) => setFormData(e.formData as unknown)}
      onSubmit={({ formData: submitted }) => {
        void ctx.actions.submit(spec.submit, submitted)
      }}
    >
      <button type="submit">{spec.submitLabel ?? 'Save'}</button>
    </Form>
  )
}

// `customValidate` is a JS function, not JSON — can't live inside a
// document's spec if that spec is meant to stay AI-safe/serializable. Same
// pattern resourcekit already uses for mutation.target/dataResolver source:
// the document references a *named* validator, the host registers the
// actual function when creating the plugin (not something an AI-authored
// document could inject on its own).
export function createRJSFPlugin(customValidators: Record<string, CustomValidator> = {}) {
  return {
    name: 'rjsf-adapter',
    kinds: [
      {
        apiVersion: 'provisr.dev/v1alpha1',
        kind: 'JSONSchemaForm',
        level: ['organism', 'template'] as string[],
        specSchema: {
          type: 'object' as const,
          additionalProperties: false,
          required: ['jsonSchema', 'submit'],
          properties: {
            jsonSchema: { type: 'object' },
            uiSchema: { type: 'object' },
            submit: { type: 'object' },
            submitLabel: { type: 'string' },
            customValidateKey: { type: 'string' },
          },
        },
        render: (resource: Resource, ctx: RenderContext) => (
          <JSONSchemaFormNode spec={resource.spec as JSONSchemaFormSpec} ctx={ctx} customValidators={customValidators} />
        ),
      },
    ],
  }
}
