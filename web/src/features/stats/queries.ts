import { useQueries, useQuery, type UseQueryResult } from '@tanstack/react-query'
import type { Span, StatsInterval } from '../../shared/filters/range'
import { fetchFeedback, fetchOverview, fetchPlanSummary, fetchTimeseries } from './api'
import type { Metric, TimeseriesResponse } from './types'

// 서버 캐시가 30초라 그보다 자주 물을 이유가 없다. 탭이 백그라운드면 갱신을 멈춘다(TanStack 기본값).
const REFRESH_MS = 30_000

export function useOverview() {
  return useQuery({
    queryKey: ['stats', 'overview'],
    queryFn: ({ signal }) => fetchOverview(signal),
    refetchInterval: REFRESH_MS,
  })
}

const timeseriesQuery = (metric: Metric, span: Span<StatsInterval>) => ({
  queryKey: ['stats', 'timeseries', metric, span.from, span.to, span.interval],
  queryFn: ({ signal }: { signal: AbortSignal }) => fetchTimeseries(metric, span, signal),
  refetchInterval: REFRESH_MS * 2,
  placeholderData: <T,>(prev: T | undefined) => prev, // 기간을 바꿀 때 차트가 비지 않게 이전 데이터를 유지
})

// 여러 지표 결과를 하나로 합친다. 하나라도 실패하면 error, 모두 오면 data 가 채워진다.
// 모듈 수준의 고정 함수여야 한다 — useQueries 는 combine 이 바뀌면 다시 계산하는데, 인라인 함수가
// 매번 새 객체를 돌려주면 렌더가 끝나지 않는다.
function combineSeries(results: UseQueryResult<TimeseriesResponse>[]) {
  return {
    data: results.every((r) => r.data) ? results.map((r) => r.data!) : undefined,
    error: results.find((r) => r.error)?.error ?? null,
    refetch: () => results.forEach((r) => void r.refetch()),
  }
}

// 여러 지표를 한 차트에 그릴 때.
export function useTimeseriesMany(metrics: Metric[], span: Span<StatsInterval>) {
  return useQueries({
    queries: metrics.map((m) => timeseriesQuery(m, span)),
    combine: combineSeries,
  })
}

export function usePlanSummary() {
  return useQuery({
    queryKey: ['stats', 'plan-summary'],
    queryFn: ({ signal }) => fetchPlanSummary(signal),
    refetchInterval: REFRESH_MS * 2,
  })
}

export function useFeedback(limit = 20) {
  return useQuery({
    queryKey: ['stats', 'feedback', limit],
    queryFn: ({ signal }) => fetchFeedback(limit, signal),
    refetchInterval: REFRESH_MS * 2,
  })
}
