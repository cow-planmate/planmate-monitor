import { KpiCard } from '../../../shared/ui/KpiCard'
import type { ApmInterval, Span } from '../../../shared/filters/range'
import { formatInt, formatPercent } from '../../../shared/lib/format'
import { useApmSeries, useErrorGroups } from '../queries'
import { shortServiceName } from '../serviceFilter'
import { errorRateTone } from '../thresholds'

const sum = (xs: number[]) => xs.reduce((a, b) => a + b, 0)

// 선택한 기간·서비스의 에러 요약.
export function ErrorKpis({ span, service }: { span: Span<ApmInterval>; service: string }) {
  const series = useApmSeries(span, service)
  const groups = useErrorGroups(span, service)

  const o = series.data?.overall
  const requests = o ? sum(o.requests) : 0
  const failures = o ? sum(o.failures) : 0
  const rate = requests > 0 ? failures / requests : null

  // 실패가 가장 많은 서비스(전체 보기일 때만)
  const worst = !service
    ? (series.data?.services ?? [])
        .map((s) => ({ name: s.service, failures: sum(s.failures) }))
        .sort((a, b) => b.failures - a.failures)[0]
    : undefined

  return (
    <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
      <KpiCard
        label="실패한 요청"
        value={formatInt(failures)}
        sub={`최근 ${span.label} · 전체 ${formatInt(requests)}건`}
        tone={failures > 0 ? 'warn' : 'ok'}
        loading={series.isPending}
        unavailable={series.isError}
      />
      <KpiCard
        label="에러율"
        value={formatPercent(rate)}
        sub={`최근 ${span.label}`}
        tone={errorRateTone(rate, requests)}
        loading={series.isPending}
        unavailable={series.isError}
      />
      <KpiCard
        label="에러 그룹"
        value={formatInt(groups.data?.items.length)}
        sub="원인별로 묶은 수"
        loading={groups.isPending}
        unavailable={groups.isError}
      />
      <KpiCard
        label={service ? '서비스' : '실패가 가장 많은 서비스'}
        value={service ? shortServiceName(service) : worst && worst.failures > 0 ? shortServiceName(worst.name) : '—'}
        sub={!service && worst ? `실패 ${formatInt(worst.failures)}건` : undefined}
        loading={series.isPending}
        unavailable={series.isError}
      />
    </div>
  )
}
