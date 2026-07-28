import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { DataBodyTemplate } from '@loykin/designkit'
import { Button } from '@/components/ui/button'
import { PageBreadcrumbTopBar } from '@/components/page-breadcrumb-topbar'
import { ProcessFormFields, formToSpec, type ProcessFormState } from '@/features/processes/ProcessFormPanel'
import { validateLifecycleHooks } from '@/components/lifecycle-hook-editor'
import { useRegisterProcess } from '@/features/processes/queries'

const initialForm: ProcessFormState = {
  name: '',
  command: '',
  workDir: '',
  env: '',
  autoRestart: false,
  instances: '',
  pidFile: '',
  priority: '',
  retryCount: '',
  retryInterval: '',
  startDuration: '',
  restartInterval: '',
  detached: false,
  dependsOn: [],
  detectors: '',
  logDir: '',
  stdoutPath: '',
  stderrPath: '',
  logMaxSize: '',
  logMaxBackups: '',
  logMaxAge: '',
  logCompress: false,
  lifecycle: {},
}

export default function ProcessRegisterPage() {
  const navigate = useNavigate()
  const [form, setForm] = useState(initialForm)
  const [error, setError] = useState<string | null>(null)
  const register = useRegisterProcess()

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    if (!form.name.trim() || !form.command.trim()) {
      setError('Name and command are required.')
      return
    }
    const lifecycleError = validateLifecycleHooks(form.lifecycle)
    if (lifecycleError) {
      setError(lifecycleError)
      return
    }
    try {
      await register.mutateAsync(formToSpec(form))
      await navigate({ to: '/processes' })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to register process.')
    }
  }

  return (
    <DataBodyTemplate
      topBar={<PageBreadcrumbTopBar items={['provisr', 'Workloads', 'Processes', 'Register']} />}
      title="Register process"
      description="Configure and start a managed process."
    >
      <form className="contents" onSubmit={(e) => void handleSubmit(e)}>
        <ProcessFormFields mode="create" form={form} setForm={setForm} />
        {error && <p className="px-4 text-sm text-destructive">{error}</p>}
        <div className="flex justify-end gap-2 px-(--designkit-page-padding-x) pb-(--designkit-page-padding-y)">
          <Button type="button" variant="outline" onClick={() => void navigate({ to: '/processes' })}>
            Cancel
          </Button>
          <Button type="submit" disabled={register.isPending}>
            Register
          </Button>
        </div>
      </form>
    </DataBodyTemplate>
  )
}
