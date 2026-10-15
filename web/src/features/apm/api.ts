import { apiGet } from '../../shared/api/client'
import type { ApmInterval, Span } from '../../shared/filters/range'
import type { ApmSeries, ApmSummary, Endpoint, ErrorGroup, JvmSeries } from './types'

type ApmSpan = Span<ApmInterval>

const rangeParams = (span: ApmSpan, service?: string) => ({
  from: span.from,
  to: span.to,
  interval: span.interval,
  service: service || undefined,
})

export const fetchApmSummary = (minutes: number, signal?: AbortSignal) =>
  apiGet<ApmSummary>('/api/apm/summary', { minutes }, signal)

export const fetchApmSeries = (span: ApmSpan, service: string, signal?: AbortSignal) =>
  apiGet<ApmSeries>('/api/apm/series', rangeParams(span, service), signal)

export const fetchEndpoints = (span: ApmSpan, service: string, limit: number, signal?: AbortSignal) =>
  apiGet<{ items: Endpoint[] }>('/api/apm/endpoints', { ...rangeParams(span, service), limit }, signal)

export const fetchErrorGroups = (span: ApmSpan, service: string, signal?: AbortSignal) =>
  apiGet<{ items: ErrorGroup[] }>('/api/apm/errors', rangeParams(span, service), signal)

export const fetchJvm = (span: ApmSpan, service: string, signal?: AbortSignal) =>
  apiGet<JvmSeries>('/api/apm/jvm', rangeParams(span, service), signal)
