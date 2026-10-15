import { Panel } from '../../../shared/ui/Panel'
import { ErrorState, Skeleton } from '../../../shared/ui/States'
import type { ApmInterval, Span } from '../../../shared/filters/range'
import { formatInt, formatMs, formatPercent } from '../../../shared/lib/format'
import { useMeta } from '../../meta/queries'
import { useEndpoints } from '../queries'
import { shortServiceName } from '../serviceFilter'
import { errorRateTone, latencyTone } from '../thresholds'
import { kibanaTransaction } from '../kibana'

const toneText = { neutral: '', ok: '', warn: 'text-warn', danger: 'text-danger' } as const

// p95 가 느린 엔드포인트 Top 10. 요청이 5건 미만인 엔드포인트는 서버가 뺀다.
export function EndpointTable({ span, service }: { span: Span<ApmInterval>; service: string }) {
  const { data, isPending, error, refetch } = useEndpoints(span, service, 10)
  const meta = useMeta().data

  return (
    <Panel title="느린 엔드포인트 Top 10" subtitle={`최근 ${span.label} · p95 기준`}>
      {error ? (
        <ErrorState message={error.message} onRetry={() => void refetch()} />
      ) : isPending ? (
        <Skeleton className="h-64" />
      ) : data.items.length === 0 ? (
        <p className="py-8 text-center text-sm text-muted">이 기간에 수집된 요청이 없다</p>
      ) : (
        <div className="overflow-x-auto">
          <table className="tabular w-full text-sm">
            <thead>
              <tr className="text-left text-xs text-muted">
                <th className="pb-2 font-normal">엔드포인트</th>
                {!service && <th className="hidden pb-2 font-normal md:table-cell">서비스</th>}
                <th className="pb-2 text-right font-normal">p95</th>
                <th className="pb-2 text-right font-normal">에러율</th>
                <th className="hidden pb-2 text-right font-normal sm:table-cell">요청</th>
              </tr>
            </thead>
            <tbody>
              {data.items.map((e) => (
                <tr key={`${e.service} ${e.name}`} className="border-t border-line">
                  <td className="max-w-0 py-2 pr-3">
                    {meta?.kibanaUrl ? (
                      <a
                        href={kibanaTransaction(meta.kibanaUrl, e.service, e.name, meta.environment)}
                        target="_blank"
                        rel="noreferrer"
                        title={`${e.name} — Kibana 에서 트레이스 보기`}
                        className="block truncate font-mono text-xs hover:text-accent hover:underline"
                      >
                        {e.name}
                      </a>
                    ) : (
                      <span className="block truncate font-mono text-xs" title={e.name}>
                        {e.name}
                      </span>
                    )}
                  </td>
                  {!service && <td className="hidden py-2 text-muted md:table-cell">{shortServiceName(e.service)}</td>}
                  <td className={`py-2 text-right ${toneText[latencyTone(e.p95Ms)]}`}>{formatMs(e.p95Ms)}</td>
                  <td className={`py-2 text-right ${toneText[errorRateTone(e.errorRate, e.requests)]}`}>
                    {formatPercent(e.errorRate)}
                  </td>
                  <td className="hidden py-2 text-right text-muted sm:table-cell">{formatInt(e.requests)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Panel>
  )
}
