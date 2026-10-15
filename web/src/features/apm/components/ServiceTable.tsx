import { Panel } from '../../../shared/ui/Panel'
import { ErrorState, Skeleton, StatusDot } from '../../../shared/ui/States'
import { formatMs, formatPercent, formatRate } from '../../../shared/lib/format'
import { useMeta } from '../../meta/queries'
import { NOW_WINDOW_MINUTES, useApmSummary } from '../queries'
import { errorRateTone, latencyTone } from '../thresholds'
import type { Tone } from '../../../shared/ui/KpiCard'
import { shortServiceName } from '../serviceFilter'
import { kibanaService } from '../kibana'

const toneText: Record<Tone, string> = { neutral: '', ok: '', warn: 'text-warn', danger: 'text-danger' }

export function ServiceTable() {
  const { data, isPending, error, refetch } = useApmSummary()
  const meta = useMeta()
  const kibana = meta.data?.kibanaUrl
  const env = meta.data?.environment

  return (
    <Panel title="서비스별 상태" subtitle={`최근 ${NOW_WINDOW_MINUTES}분 · APM 에이전트가 붙은 서비스`}>
      {error ? (
        <ErrorState message={error.message} onRetry={() => void refetch()} />
      ) : isPending ? (
        <Skeleton className="h-40" />
      ) : !data.services?.length ? (
        <p className="py-8 text-center text-sm text-muted">최근 {NOW_WINDOW_MINUTES}분 동안 수집된 요청이 없다</p>
      ) : (
        <table className="tabular w-full text-sm">
          <thead>
            <tr className="text-left text-xs text-muted">
              <th className="pb-2 font-normal">서비스</th>
              <th className="pb-2 text-right font-normal">p95</th>
              <th className="pb-2 text-right font-normal">에러율</th>
              <th className="hidden pb-2 text-right font-normal sm:table-cell">처리량</th>
            </tr>
          </thead>
          <tbody>
            {data.services.map((s) => {
              const errTone = errorRateTone(s.errorRate, s.requests)
              const name = shortServiceName(s.service)
              return (
                <tr key={s.service} className="border-t border-line">
                  <td className="py-2">
                    <span className="flex items-center gap-2">
                      <StatusDot ok={errTone !== 'danger'} />
                      {kibana ? (
                        <a
                          href={kibanaService(kibana, s.service, env)}
                          target="_blank"
                          rel="noreferrer"
                          className="hover:text-accent hover:underline"
                          title="Kibana 에서 보기"
                        >
                          {name}
                        </a>
                      ) : (
                        name
                      )}
                    </span>
                  </td>
                  <td className={`py-2 text-right ${toneText[latencyTone(s.p95Ms)]}`}>{formatMs(s.p95Ms)}</td>
                  <td className={`py-2 text-right ${toneText[errTone]}`}>{formatPercent(s.errorRate)}</td>
                  <td className="hidden py-2 text-right text-muted sm:table-cell">{formatRate(s.rpm)}</td>
                </tr>
              )
            })}
          </tbody>
        </table>
      )}
    </Panel>
  )
}
