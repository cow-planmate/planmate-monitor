import { useState } from 'react'
import { Panel } from '../../../shared/ui/Panel'
import { ErrorState, Skeleton } from '../../../shared/ui/States'
import type { ApmInterval, Span } from '../../../shared/filters/range'
import { formatAgo, formatDateTime, formatInt } from '../../../shared/lib/format'
import { useMeta } from '../../meta/queries'
import { useErrorGroups } from '../queries'
import { shortServiceName } from '../serviceFilter'
import { kibanaError } from '../kibana'
import type { ErrorGroup } from '../types'

const shortType = (t: string) => t.split('.').pop() ?? t

// 에러 그룹: 같은 원인(APM grouping_key)끼리 묶어 발생 횟수가 많은 순서로 보여준다.
export function ErrorGroupTable({ span, service }: { span: Span<ApmInterval>; service: string }) {
  const { data, isPending, error, refetch } = useErrorGroups(span, service)
  const [open, setOpen] = useState<string | null>(null)

  return (
    <Panel title="에러 그룹" subtitle={`최근 ${span.label} · 발생 횟수순 · 행을 누르면 상세`}>
      {error ? (
        <ErrorState message={error.message} onRetry={() => void refetch()} />
      ) : isPending ? (
        <Skeleton className="h-64" />
      ) : data.items.length === 0 ? (
        <p className="py-8 text-center text-sm text-muted">이 기간에 수집된 에러가 없다</p>
      ) : (
        <ul className="divide-y divide-line">
          {data.items.map((g) => (
            <ErrorRow
              key={g.groupingKey + g.service}
              group={g}
              showService={!service}
              open={open === g.groupingKey}
              onToggle={() => setOpen(open === g.groupingKey ? null : g.groupingKey)}
            />
          ))}
        </ul>
      )}
    </Panel>
  )
}

function ErrorRow({
  group: g,
  showService,
  open,
  onToggle,
}: {
  group: ErrorGroup
  showService: boolean
  open: boolean
  onToggle: () => void
}) {
  const meta = useMeta().data

  return (
    <li>
      <button
        type="button"
        onClick={onToggle}
        aria-expanded={open}
        className="flex w-full items-start gap-3 py-2.5 text-left hover:bg-surface-2/50"
      >
        <span className="tabular w-14 shrink-0 text-right text-sm font-semibold text-danger">{formatInt(g.count)}</span>
        <span className="min-w-0 flex-1">
          <span className="block truncate text-sm">{g.message || '(메시지 없음)'}</span>
          <span className="mt-0.5 block truncate text-xs text-muted">
            <span className="font-mono">{shortType(g.type)}</span>
            {showService && <> · {shortServiceName(g.service)}</>}
            {g.transaction && <> · {g.transaction}</>}
          </span>
        </span>
        <span className="hidden shrink-0 text-xs text-muted sm:block" title={formatDateTime(g.lastSeen)}>
          {formatAgo(g.lastSeen)}
        </span>
      </button>
      {open && (
        <dl className="mb-3 ml-[4.25rem] grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 rounded-md bg-surface-2 p-3 text-xs">
          <dt className="text-muted">예외</dt>
          <dd className="break-all font-mono">{g.type}</dd>
          <dt className="text-muted">발생 위치</dt>
          <dd className="break-all font-mono">{g.culprit || '—'}</dd>
          <dt className="text-muted">처음 / 마지막</dt>
          <dd>
            {formatDateTime(g.firstSeen)} / {formatDateTime(g.lastSeen)}
          </dd>
          <dt className="text-muted">메시지</dt>
          <dd className="break-words">{g.message}</dd>
          {meta?.kibanaUrl && (
            <dd className="col-span-2 pt-1">
              <a
                href={kibanaError(meta.kibanaUrl, g.service, g.groupingKey, meta.environment)}
                target="_blank"
                rel="noreferrer"
                className="text-accent hover:underline"
              >
                Kibana 에서 스택트레이스 보기 →
              </a>
            </dd>
          )}
        </dl>
      )}
    </li>
  )
}
