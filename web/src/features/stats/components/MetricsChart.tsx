import { useMemo } from 'react'
import { Panel } from '../../../shared/ui/Panel'
import { ErrorState, Skeleton } from '../../../shared/ui/States'
import { TimeSeriesChart, type ChartSeries } from '../../../shared/charts/TimeSeriesChart'
import type { Span, StatsInterval } from '../../../shared/filters/range'
import { useTimeseriesMany } from '../queries'
import type { Metric } from '../types'

export interface MetricLine {
  metric: Metric
  name: string
  type: 'bar' | 'line'
  axis?: 0 | 1
  stack?: string
}

interface Props {
  title: string
  subtitle?: string
  lines: MetricLine[]
  span: Span<StatsInterval>
}

// 사용자 통계 지표 여러 개를 한 차트에 그린다. 지표 정의는 서버(internal/stats/model.go)에 있다.
export function MetricsChart({ title, subtitle, lines, span }: Props) {
  const { data, error, refetch } = useTimeseriesMany(
    lines.map((l) => l.metric),
    span,
  )

  const chart = useMemo(() => {
    if (!data) return null
    const series: ChartSeries[] = lines.map((l, i) => ({
      name: l.name,
      type: l.type,
      axis: l.axis,
      stack: l.stack,
      data: data[i].points.map((p) => p.value),
    }))
    return { buckets: data[0].points.map((p) => p.t), series }
  }, [data, lines])

  return (
    <Panel title={title} subtitle={subtitle ?? `최근 ${span.label}`}>
      {error ? (
        <ErrorState message={error.message} onRetry={refetch} />
      ) : chart ? (
        <TimeSeriesChart buckets={chart.buckets} series={chart.series} interval={span.interval} />
      ) : (
        <Skeleton className="h-[280px]" />
      )}
    </Panel>
  )
}
