// PoC only (branch poc/resourcekit-forms) — shared local resourcekit kinds
// and resolvers for provisr, used by *RK pages. Kept separate from
// resourcekit itself: these are gap-fills for kinds/resolvers the published
// package doesn't have yet (see resourcekit/docs/provisr-poc-findings.md).
import { useState } from 'react'
import type { DataResolver, Resource } from '@loykin/resourcekit'
import type { RenderContext } from '@loykin/resourcekit/react'
import { Checkbox } from '@/components/ui/checkbox'
import { Textarea } from '@/components/ui/textarea'
import { LifecycleHookEditor, hasLifecycleHooks } from '@/components/lifecycle-hook-editor'
import type { LifecycleHooks } from '@/components/lifecycle-hooks'
import { apiFetch } from '@/lib/api'

// The env-lines <-> string[] conversion itself is domain logic (provisr's
// KEY=VALUE-per-line convention), not something a UI kind could know about
// — kind-ifying the Textarea field fixes the *markup* duplication seen
// across ProcessFormPanel/CronJobFormPanel, but this parsing duplication is
// a separate axis and needs a plain shared function regardless of
// resourcekit. Both *RK mutation resolvers import this one.
export function parseEnvLines(value: string): string[] {
  return value
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
}

function getValueAtPath(value: unknown, path: string | undefined): unknown {
  if (!path || typeof value !== 'object' || value === null) return undefined
  return path.split('.').reduce<unknown>((current, part) => {
    if (typeof current !== 'object' || current === null) return undefined
    return (current as Record<string, unknown>)[part]
  }, value)
}

// #4 in the findings doc: no Checkbox/checkbox-group kind in resourcekit yet.
// Wraps the same @base-ui/react-backed Checkbox provisr's own UserFormFields
// uses — its hidden native <input type="checkbox"> participates in
// ResourceForm's FormData collection like any other field. `fieldRef` (an
// array field on the record) prefills checked state for edit mode, the same
// convention InputControl uses for text fields.
interface CheckboxGroupSpec {
  label?: string
  name: string
  options: Array<{ label: string; value: string }>
  defaultValues?: string[]
  fieldRef?: string
}

function CheckboxGroupNode({ spec, ctx }: { spec: CheckboxGroupSpec; ctx: RenderContext }) {
  const fromRecord = spec.fieldRef !== undefined ? getValueAtPath(ctx.record, spec.fieldRef) : undefined
  const selected = Array.isArray(fromRecord) ? fromRecord.map(String) : spec.defaultValues
  return (
    <div className="flex flex-col gap-2">
      {spec.options.map((option) => (
        <label key={option.value} className="flex items-center gap-2 text-sm">
          <Checkbox name={spec.name} value={option.value} defaultChecked={selected?.includes(option.value)} />
          {option.label}
        </label>
      ))}
    </div>
  )
}

// #3 in the findings doc: no Textarea kind in resourcekit yet. Unlike
// Checkbox, provisr's Textarea is a plain native <textarea> — no hidden-input
// trick needed, it participates in ResourceForm's FormData collection as
// long as `name` is set.
interface TextareaFieldSpec {
  name: string
  placeholder?: string
  rows?: number
  monospace?: boolean
  fieldRef?: string
}

function TextareaFieldNode({ spec, ctx }: { spec: TextareaFieldSpec; ctx: RenderContext }) {
  const fromRecord = spec.fieldRef !== undefined ? getValueAtPath(ctx.record, spec.fieldRef) : undefined
  const defaultValue = Array.isArray(fromRecord) ? fromRecord.join('\n') : (fromRecord as string | undefined)
  return (
    <Textarea
      name={spec.name}
      placeholder={spec.placeholder}
      rows={spec.rows ?? 4}
      defaultValue={defaultValue}
      className={spec.monospace ? 'font-mono text-xs' : undefined}
    />
  )
}

// #5 in the findings doc: no Select kind in resourcekit yet. Like Textarea,
// a native <select> participates in ResourceForm's FormData collection with
// no special trick — just needs `name`.
interface SelectFieldSpec {
  name: string
  options: Array<{ label: string; value: string }>
  defaultValue?: string
  fieldRef?: string
}

const selectClassName =
  'h-8 w-full rounded-lg border border-input bg-transparent px-2.5 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30'

function SelectFieldNode({ spec, ctx }: { spec: SelectFieldSpec; ctx: RenderContext }) {
  const fromRecord = spec.fieldRef !== undefined ? getValueAtPath(ctx.record, spec.fieldRef) : undefined
  const defaultValue = (fromRecord as string | undefined) ?? spec.defaultValue
  return (
    <select name={spec.name} defaultValue={defaultValue} className={selectClassName}>
      {spec.options.map((option) => (
        <option key={option.value} value={option.value}>
          {option.label}
        </option>
      ))}
    </select>
  )
}

// #6 in the findings doc — the case flagged as most likely to actually
// break ResourceForm/FormView's flat FormData submit model: a repeatable
// group of structured objects (per-phase arrays of
// {name, command, work_dir, env[], timeout, failure_mode, run_mode}), not a
// single value. Native FormData has no nested-object encoding, so this kind
// doesn't try to decompose the editor into many named fields — it reuses
// the *real* LifecycleHookEditor component (own internal add/remove/edit
// state, unchanged from the hand-written form) for the UI, then serializes
// its whole value into one hidden <input type="hidden"> field for FormData
// to pick up as a single JSON string. The mutation resolver JSON.parses it
// back. This works, but it means the field's "spec" is not really
// schema-validatable JSON at the sub-field level — validateResource can
// only ever see "some string," not the hook structure inside it.
interface LifecycleHooksFieldSpec {
  name: string
  fieldRef?: string
}

function LifecycleHooksFieldNode({ spec, ctx }: { spec: LifecycleHooksFieldSpec; ctx: RenderContext }) {
  const fromRecord = spec.fieldRef !== undefined ? getValueAtPath(ctx.record, spec.fieldRef) : undefined
  const [value, setValue] = useState<LifecycleHooks>(() => (fromRecord as LifecycleHooks | undefined) ?? {})
  return (
    <div>
      <LifecycleHookEditor value={value} onChange={setValue} />
      <input type="hidden" name={spec.name} value={hasLifecycleHooks(value) ? JSON.stringify(value) : ''} readOnly />
    </div>
  )
}

export function createProvisrKindsPlugin() {
  return {
    name: 'provisr-custom-kinds',
    kinds: [
      {
        apiVersion: 'provisr.dev/v1alpha1',
        kind: 'CheckboxGroup',
        level: ['leaf'] as string[],
        specSchema: {
          type: 'object' as const,
          additionalProperties: false,
          required: ['name', 'options'],
          properties: {
            label: { type: 'string' },
            name: { type: 'string' },
            options: {
              type: 'array',
              items: {
                type: 'object',
                required: ['label', 'value'],
                properties: { label: { type: 'string' }, value: { type: 'string' } },
              },
            },
            defaultValues: { type: 'array', items: { type: 'string' } },
            fieldRef: { type: 'string' },
          },
        },
        render: (resource: Resource, ctx: RenderContext) => (
          <CheckboxGroupNode spec={resource.spec as CheckboxGroupSpec} ctx={ctx} />
        ),
      },
      {
        apiVersion: 'provisr.dev/v1alpha1',
        kind: 'TextareaField',
        level: ['leaf'] as string[],
        specSchema: {
          type: 'object' as const,
          additionalProperties: false,
          required: ['name'],
          properties: {
            name: { type: 'string' },
            placeholder: { type: 'string' },
            rows: { type: 'number' },
            monospace: { type: 'boolean' },
            fieldRef: { type: 'string' },
          },
        },
        render: (resource: Resource, ctx: RenderContext) => (
          <TextareaFieldNode spec={resource.spec as TextareaFieldSpec} ctx={ctx} />
        ),
      },
      {
        apiVersion: 'provisr.dev/v1alpha1',
        kind: 'Select',
        level: ['leaf'] as string[],
        specSchema: {
          type: 'object' as const,
          additionalProperties: false,
          required: ['name', 'options'],
          properties: {
            name: { type: 'string' },
            options: {
              type: 'array',
              items: {
                type: 'object',
                required: ['label', 'value'],
                properties: { label: { type: 'string' }, value: { type: 'string' } },
              },
            },
            defaultValue: { type: 'string' },
            fieldRef: { type: 'string' },
          },
        },
        render: (resource: Resource, ctx: RenderContext) => (
          <SelectFieldNode spec={resource.spec as SelectFieldSpec} ctx={ctx} />
        ),
      },
      {
        apiVersion: 'provisr.dev/v1alpha1',
        kind: 'LifecycleHooksField',
        level: ['leaf'] as string[],
        specSchema: {
          type: 'object' as const,
          additionalProperties: false,
          required: ['name'],
          properties: {
            name: { type: 'string' },
            fieldRef: { type: 'string' },
          },
        },
        render: (resource: Resource, ctx: RenderContext) => (
          <LifecycleHooksFieldNode spec={resource.spec as LifecycleHooksFieldSpec} ctx={ctx} />
        ),
      },
    ],
  }
}

// Auth investigation finding: resourcekit's built-in `restResolver` does a
// raw fetch(url, {headers: b.headers}) with headers taken verbatim from the
// JSON binding — no hook for a per-request, rotating credential. provisr's
// auth is a client-held JWT (localStorage) attached per-request by
// apiFetch(), plus global 401 -> logout handling (onAuthExpired). A resource
// document can't safely embed that token as a static `headers` value (it
// would go stale on refresh/logout, and baking a credential into a JSON
// document is exactly what resourcekit's own `connections` feature exists
// to avoid). Fix: don't use the built-in `rest` resolver for
// provisr-authenticated endpoints — register this resolver instead, which
// delegates to the app's own already-authenticated fetch wrapper. No
// resourcekit core change is required for this; it's the same bridging
// pattern already used for mutations (provisr-create-user).
export const provisrRestResolver: DataResolver = async (binding) => {
  const b = binding as unknown as { path: string }
  const json = await apiFetch<unknown>(b.path)
  return Array.isArray(json) ? (json as Record<string, unknown>[]) : [json as Record<string, unknown>]
}
