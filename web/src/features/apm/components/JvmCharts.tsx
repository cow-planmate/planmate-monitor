import { useMemo } from 'react'
import { Panel } from '../../../shared/ui/Panel'
import { ErrorState, Skeleton } from '../../../shared/ui/States'
import { TimeSeriesChart, type ChartSeries } from '../../../shared/charts/TimeSeriesChart'
import type { ApmInterval, Span } from '../../../shared/filters/range'
import { formatInt, formatMs } from '../../../shared/lib/format'
import { useJvm } from '../queries'
import { shortServiceName } from '../serviceFilter'

const formatMB = (v: number) => `${Math.round(v)}MB`

// JVM 상태(Java 에이전트 메트릭). 서비스 하나를 골랐을 때만 보인다.
export function JvmCharts({ span, service }: { span: Span<ApmInterval>; service: string }) {
  const { data, error, refetch } = useJvm(span, service)

  const charts = useMemo(() => {
    if (!data) return null
    const heap: ChartSeries[] = [
      { name: '힙 사용(평균)', type: 'line', data: data.heapUsedMB, color: '--accent' },
      { name: '힙 최대', type: 'line', data: data.heapMaxMB, color: '--danger' },
    ]
    const gc: ChartSeries[] = [
      { name: 'GC 시간', type: 'bar', data: data.gcTimeMs, color: '--warn' },
      { name: '스레드', type: 'line', data: data.threads, axis: 1, color: '--accent-2' },
    ]
    return { heap, gc }
  }, [data])

  if (!service) {
    return (
      <Panel title="JVM" subtitle="서비스를 하나 고르면 힙·GC·스레드를 보여준다">
        <p className="py-6 text-center text-sm text-muted">위에서 서비스를 선택한다</p>
      </Panel>
    )
  }

  const empty = data && data.heapUsedMB.every((v) => v == null)
  const body = (content: (c: NonNullable<typeof charts>) => React.ReactNode) =>
    error ? (
      <ErrorState message={error.message} onRetry={() => void refetch()} />
    ) : empty ? (
      <p className="py-8 text-center text-sm text-muted">이 기간에 수집된 JVM 메트릭이 없다</p>
    ) : data && charts ? (
      content(charts)
    ) : (
      <Skeleton className="h-[240px]" />
    )

  const name = shortServiceName(service)
  return (
    <div className="grid gap-4 lg:grid-cols-2">
      <Panel title="JVM 힙" subtitle={`${name} · 버킷 평균 사용량과 최대치`}>
        {body((c) => <TimeSeriesChart buckets={data!.buckets} series={c.heap} interval={data!.interval} format={formatMB} height={240} />)}
      </Panel>
      <Panel title="GC · 스레드" subtitle={`${name} · 막대는 구간별 GC 시간, 선은 스레드 수(오른쪽 축)`}>
        {body((c) => (
          <TimeSeriesChart
            buckets={data!.buckets}
            series={c.gc}
            interval={data!.interval}
            format={formatMs}
            format2={formatInt}
            height={240}
          />
        ))}
      </Panel>
    </div>
  )
}
