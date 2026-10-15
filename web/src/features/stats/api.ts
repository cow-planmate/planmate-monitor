import { apiGet } from '../../shared/api/client'
import type { Span, StatsInterval } from '../../shared/filters/range'
import type { Feedback, Metric, Overview, PlanSummary, TimeseriesResponse } from './types'

export const fetchOverview = (signal?: AbortSignal) => apiGet<Overview>('/api/stats/overview', undefined, signal)

export const fetchTimeseries = (metric: Metric, span: Span<StatsInterval>, signal?: AbortSignal) =>
  apiGet<TimeseriesResponse>(
    '/api/stats/timeseries',
    { metric, from: span.from, to: span.to, interval: span.interval },
    signal,
  )

export const fetchPlanSummary = (signal?: AbortSignal) =>
  apiGet<PlanSummary>('/api/stats/plans/summary', undefined, signal)

export const fetchFeedback = (limit: number, signal?: AbortSignal) =>
  apiGet<{ items: Feedback[] }>('/api/stats/feedback', { limit }, signal)
