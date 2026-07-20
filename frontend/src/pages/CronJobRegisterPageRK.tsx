// PoC only (branch poc/resourcekit-forms) — tests the actual motivating
// duplication: ProcessFormPanel.tsx and CronJobFormPanel.tsx both hand-roll
// the same env-textarea Row and copy-paste the same
// `env.split('\n').map(trim).filter(Boolean)` parsing. This page reuses the
// *same* TextareaField kind (and the shared parseEnvLines helper) that a
// ProcessRegisterPageRK would use — the point is whether that reuse actually
// happens, not just whether one page in isolation works.
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
import { createProvisrKindsPlugin, parseEnvLines } from '@/lib/resourcekit-provisr'
import { createCronJob } from '@/features/cronjobs/api'
import type { CronJobSpec } from '@/features/cronjobs/types'
import type { LifecycleHooks } from '@/components/lifecycle-hooks'

function inputRow(label: string, required: boolean | undefined, inputSpec: Record<string, unknown>): Resource {
  return {
    apiVersion: 'resourcekit.dev/v1alpha1',
    kind: 'DataBodyRow',
    spec: { label, required },
    slots: [{ items: [{ apiVersion: 'resourcekit.dev/v1alpha1', kind: 'InputControl', spec: inputSpec }] }],
  }
}

const resource: Resource = {
  apiVersion: 'resourcekit.dev/v1alpha1',
  kind: 'ResourceForm',
  spec: {
    submit: {
      mutation: { target: 'provisr-create-cronjob' },
      onSuccess: [{ kind: 'emit', event: 'provisr.cronjob.created' }],
    },
    submitLabel: 'Create',
    successMessage: 'Cron job created',
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
                inputRow('Name', true, { name: 'name' }),
                inputRow('Schedule', true, { name: 'schedule', placeholder: 'e.g. 0 */6 * * * or @every 1h' }),
                inputRow('Command', true, { name: 'command', placeholder: 'e.g. /usr/bin/backup.sh' }),
                inputRow('Working directory', false, { name: 'workDir', placeholder: '(optional) absolute path' }),
                {
                  apiVersion: 'resourcekit.dev/v1alpha1',
                  kind: 'DataBodyRow',
                  spec: { label: 'Environment', description: 'One KEY=VALUE per line' },
                  slots: [
                    {
                      items: [
                        {
                          apiVersion: 'provisr.dev/v1alpha1',
                          kind: 'TextareaField',
                          spec: { name: 'env', monospace: true, rows: 4 },
                        },
                      ],
                    },
                  ],
                },
                {
                  apiVersion: 'resourcekit.dev/v1alpha1',
                  kind: 'DataBodyRow',
                  spec: { label: 'Concurrency policy' },
                  slots: [
                    {
                      items: [
                        {
                          apiVersion: 'provisr.dev/v1alpha1',
                          kind: 'Select',
                          spec: {
                            name: 'concurrencyPolicy',
                            defaultValue: 'Allow',
                            options: [
                              { label: 'Allow — run concurrently', value: 'Allow' },
                              { label: 'Forbid — skip if previous still running', value: 'Forbid' },
                              { label: 'Replace — cancel previous, start new', value: 'Replace' },
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
                  spec: { label: 'Cronjob lifecycle hooks', description: 'Commands merged into every Job created by this schedule' },
                  slots: [
                    {
                      items: [
                        {
                          apiVersion: 'provisr.dev/v1alpha1',
                          kind: 'LifecycleHooksField',
                          spec: { name: 'lifecycle' },
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

export default function CronJobRegisterPageRK() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const registry = useMemo(() => {
    const createCronJobMutationResolver: MutationResolver = async (_binding, payload) => {
      const body = payload as Record<string, unknown>
      const env = parseEnvLines(String(body.env ?? ''))
      const lifecycleRaw = String(body.lifecycle ?? '').trim()
      const lifecycle: LifecycleHooks | undefined = lifecycleRaw ? (JSON.parse(lifecycleRaw) as LifecycleHooks) : undefined
      const spec: CronJobSpec = {
        name: String(body.name ?? '').trim(),
        schedule: String(body.schedule ?? '').trim(),
        job_template: {
          command: String(body.command ?? '').trim(),
          work_dir: body.workDir ? String(body.workDir).trim() || undefined : undefined,
          env: env.length > 0 ? env : undefined,
        },
        concurrency_policy: (body.concurrencyPolicy as CronJobSpec['concurrency_policy']) ?? 'Allow',
        lifecycle,
      }
      await createCronJob(spec)
      await queryClient.invalidateQueries({ queryKey: ['cronjobs'] })
      return spec
    }

    const reg = createRegistry<KindRenderFn>()
    reg.use(createDesignKitPlugin())
    reg.use(createProvisrKindsPlugin())
    reg.use({
      name: 'provisr-mutations',
      mutationResolvers: { 'provisr-create-cronjob': createCronJobMutationResolver },
    })
    return reg
  }, [queryClient])

  return (
    <div className="flex h-full flex-col">
      <DataBodyTemplate
        title="Create cron job (resourcekit PoC)"
        contentClassName="flex-1"
        actions={
          <Button type="button" variant="ghost" onClick={() => void navigate({ to: '/cronjobs' })}>
            Cancel
          </Button>
        }
      >
        <ResourceRenderer
          registry={registry}
          resource={resource}
          onEvent={(event) => {
            if (event === 'provisr.cronjob.created') void navigate({ to: '/cronjobs' })
          }}
          renderError={(error) => <p className="px-4 text-sm text-destructive">{String(error)}</p>}
        />
      </DataBodyTemplate>
    </div>
  )
}
