import { useId, useState } from 'react';
import { Ban, Hand, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import {
  useApprovalPolicies,
  useClearApprovalPolicy,
  usePolicyTools,
  useSetApprovalPolicy,
  type ApprovalPolicy,
  type ToolPolicy,
} from '../approvals/useApprovalPolicies';
import { Spinner } from '../components/Spinner';
import { PolicyToolPicker } from './PolicyToolPicker';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Select } from '@/components/ui/select';

// ToolPoliciesPanel is the narrowing half of the standing approvals (prd.md §5, 2026-10-09):
// where an operator says "always ask me before this tool" or "never run it". Like the grants
// above it, it lists only the authenticated principal's own policies; setting one on another
// identity is the CLI's job. The tool is picked from the agent's own tools, MCP mounts
// included, and the action from the verbs the gateway keys that tool by, if it has any.
export function ToolPoliciesPanel() {
  const { t } = useTranslation();
  const policiesQuery = useApprovalPolicies();
  const setPolicy = useSetApprovalPolicy();
  const clearPolicy = useClearApprovalPolicy();
  const policies = policiesQuery.data ?? [];

  return (
    <section aria-labelledby="settings-tool-policies" className="flex flex-col gap-4">
      <div className="flex flex-col gap-2">
        <h3 id="settings-tool-policies" className="text-[17px] font-semibold text-text">
          {t('settings.toolPolicies.heading')}
        </h3>
        <p className="max-w-3xl text-[15px] leading-relaxed text-text-muted">
          {t('settings.toolPolicies.body')}
        </p>
      </div>

      {policiesQuery.isPending ? <Spinner /> : null}

      {policiesQuery.isError ? (
        <Alert variant="destructive">
          <AlertDescription>{t('settings.toolPolicies.error')}</AlertDescription>
        </Alert>
      ) : null}

      {setPolicy.isError || clearPolicy.isError ? (
        <Alert variant="destructive">
          <AlertDescription>{t('settings.toolPolicies.saveError')}</AlertDescription>
        </Alert>
      ) : null}

      {!policiesQuery.isPending && !policiesQuery.isError && policies.length === 0 ? (
        <p className="text-[15px] text-text-muted">{t('settings.toolPolicies.empty')}</p>
      ) : null}

      {policies.length > 0 ? (
        <ul className="flex flex-col gap-2">
          {policies.map((policy) => (
            <PolicyRow
              key={`${policy.tool} ${policy.action}`}
              policy={policy}
              busy={clearPolicy.isPending}
              onClear={() => {
                clearPolicy.mutate({ tool: policy.tool, action: policy.action });
              }}
            />
          ))}
        </ul>
      ) : null}

      <PolicyForm
        busy={setPolicy.isPending}
        onSubmit={(vars) => {
          setPolicy.mutate(vars);
        }}
      />
    </section>
  );
}

interface PolicyRowProps {
  readonly policy: ApprovalPolicy;
  readonly busy: boolean;
  readonly onClear: () => void;
}

function PolicyRow({ policy, busy, onClear }: PolicyRowProps) {
  const { t } = useTranslation();
  const Icon = policy.policy === 'deny' ? Ban : Hand;
  return (
    <li className="flex items-center justify-between gap-4 rounded-md border border-border bg-surface px-3 py-2.5">
      <span className="flex min-w-0 items-center gap-2.5">
        <Icon
          aria-hidden="true"
          className={
            policy.policy === 'deny'
              ? 'size-4 shrink-0 text-danger'
              : 'size-4 shrink-0 text-accent-text'
          }
        />
        <span className="flex min-w-0 flex-col">
          <span className="truncate font-mono text-[14px] text-text">{policy.subject}</span>
          <span className="text-[12.5px] text-text-muted">
            {t(`settings.toolPolicies.policy.${policy.policy}`)}
          </span>
        </span>
      </span>
      <Button
        type="button"
        variant="outline"
        size="sm"
        disabled={busy}
        onClick={onClear}
        className="shrink-0 text-[0.8125rem]"
      >
        <X aria-hidden="true" />
        {t('settings.toolPolicies.clear')}
      </Button>
    </li>
  );
}

interface PolicyFormProps {
  readonly busy: boolean;
  readonly onSubmit: (vars: { tool: string; action: string; policy: ToolPolicy }) => void;
}

function PolicyForm({ busy, onSubmit }: PolicyFormProps) {
  const { t } = useTranslation();
  const toolId = useId();
  const actionId = useId();
  const policyId = useId();
  const toolsQuery = usePolicyTools();
  const tools = toolsQuery.data ?? [];
  const [tool, setTool] = useState('');
  const [action, setAction] = useState('');
  const [policy, setPolicy] = useState<ToolPolicy>('ask');
  const actions = tools.find((candidate) => candidate.name === tool)?.actions ?? [];

  return (
    <form
      onSubmit={(event) => {
        event.preventDefault();
        if (tool === '') return;
        onSubmit({ tool, action, policy });
        setTool('');
        setAction('');
      }}
      className="grid gap-3 sm:grid-cols-[2fr_1.5fr_1fr_auto] sm:items-end"
    >
      {toolsQuery.isError ? (
        <Alert variant="destructive" className="sm:col-span-4">
          <AlertDescription>{t('settings.toolPolicies.toolsError')}</AlertDescription>
        </Alert>
      ) : null}
      <div className="flex flex-col gap-1.5">
        <Label htmlFor={toolId}>{t('settings.toolPolicies.toolLabel')}</Label>
        <PolicyToolPicker
          id={toolId}
          tools={tools}
          value={tool}
          onChange={(name) => {
            setTool(name);
            setAction('');
          }}
        />
      </div>
      <div className="flex flex-col gap-1.5">
        <Label htmlFor={actionId}>{t('settings.toolPolicies.actionLabel')}</Label>
        <Select
          id={actionId}
          value={action}
          disabled={actions.length === 0}
          onValueChange={setAction}
          options={[
            {
              value: '',
              label:
                actions.length === 0
                  ? t('settings.toolPolicies.actionNone')
                  : t('settings.toolPolicies.actionAll'),
            },
            ...actions.map((verb) => ({ value: verb, label: verb })),
          ]}
          className="font-mono"
        />
      </div>
      <div className="flex flex-col gap-1.5">
        <Label htmlFor={policyId}>{t('settings.toolPolicies.policyLabel')}</Label>
        <Select
          id={policyId}
          value={policy}
          onValueChange={(next) => {
            setPolicy(next === 'deny' ? 'deny' : 'ask');
          }}
          options={[
            { value: 'ask', label: t('settings.toolPolicies.policy.ask') },
            { value: 'deny', label: t('settings.toolPolicies.policy.deny') },
          ]}
        />
      </div>
      <Button type="submit" disabled={busy || tool === ''}>
        {t('settings.toolPolicies.save')}
      </Button>
    </form>
  );
}
