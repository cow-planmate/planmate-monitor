import { useMemo } from 'react'
import { Panel } from '../../../shared/ui/Panel'
import { ErrorState, Skeleton } from '../../../shared/ui/States'
import { TimeSeriesChart, type ChartSeries } from '../../../shared/charts/TimeSeriesChart'
import type { ApmInterval, Span } from '../../../shared/filters/range'
import { formatInt, formatMs } from '../../../shared/lib/format'
import { useApmSeries } from '../queries'
import { shortServiceName } from '../serviceFilter'

// 응답시간 p50/p95/p99(왼쪽 축)와 요청 수(오른쪽 축, 막대).
export function LatencyChart({ span, service }: { span: Span<ApmInterval>; service: string }) {
  const { data, error, refetch } = useApmSeries(span, service)

  const series = useMemo<ChartSeries[] | null>(() => {
    if (!data) return null
    const m = data.overall
    return [
      { name: '요청 수', type: 'bar', data: m.requests, axis: 1, color: '--border' },
      { name: 'p50', type: 'line', data: m.p50Ms, color: '--ok' },
      { name: 'p95', type: 'line', data: m.p95Ms, color: '--accent' },
      { name: 'p99', type: 'line', data: m.p99Ms, color: '--warn' },
    ]
  }, [data])

  return (
    <Panel
      title="응답시간 · 처리량"
      subtitle={`최근 ${span.label} · ${service ? shortServiceName(service) : '전체 서비스'} · 헬스체크 제외`}
    >
      {error ? (
        <ErrorState message={error.message} onRetry={() => void refetch()} />
      ) : data && series ? (
        <TimeSeriesChart
          buckets={data.buckets}
          series={series}
          interval={data.interval}
          format={formatMs}
          format2={formatInt}
          height={300}
        />
      ) : (
        <Skeleton className="h-[300px]" />
      )}
    </Panel>
  )
}
