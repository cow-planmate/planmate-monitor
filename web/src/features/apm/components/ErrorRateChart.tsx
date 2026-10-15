import { useMemo } from 'react'
import { Panel } from '../../../shared/ui/Panel'
import { ErrorState, Skeleton } from '../../../shared/ui/States'
import { TimeSeriesChart, type ChartSeries } from '../../../shared/charts/TimeSeriesChart'
import type { ApmInterval, Span } from '../../../shared/filters/range'
import { formatInt, formatPercent } from '../../../shared/lib/format'
import { useApmSeries } from '../queries'
import { shortServiceName } from '../serviceFilter'

// 전체를 보면 서비스별 에러율 선, 서비스 하나를 보면 실패 건수(막대)와 에러율(선).
export function ErrorRateChart({ span, service }: { span: Span<ApmInterval>; service: string }) {
  const { data, error, refetch } = useApmSeries(span, service)

  const series = useMemo<ChartSeries[] | null>(() => {
    if (!data) return null
    if (service) {
      return [
        { name: '실패 건수', type: 'bar', data: data.overall.failures, axis: 1, color: '--danger' },
        { name: '에러율', type: 'line', data: data.overall.errorRate, color: '--warn' },
      ]
    }
    return (data.services ?? []).map((s) => ({
      name: shortServiceName(s.service),
      type: 'line' as const,
      data: s.errorRate,
    }))
  }, [data, service])

  const fmtRate = (v: number) => formatPercent(v)

  return (
    <Panel title="에러율 추이" subtitle={`최근 ${span.label} · ${service ? shortServiceName(service) : '서비스별'}`}>
      {error ? (
        <ErrorState message={error.message} onRetry={() => void refetch()} />
      ) : data && series ? (
        <TimeSeriesChart
          buckets={data.buckets}
          series={series}
          interval={data.interval}
          format={fmtRate}
          format2={formatInt}
          height={280}
        />
      ) : (
        <Skeleton className="h-[280px]" />
      )}
    </Panel>
  )
}
