import { useQuery } from '@tanstack/react-query'
import type { ApmInterval, Span } from '../../shared/filters/range'
import { fetchApmSeries, fetchApmSummary, fetchEndpoints, fetchErrorGroups, fetchJvm } from './api'

// 개요 화면의 "지금 괜찮은가" 는 최근 15분으로 본다.
export const NOW_WINDOW_MINUTES = 15

const REFRESH_MS = 30_000
const keep = <T,>(prev: T | undefined) => prev // 기간·서비스를 바꿀 때 화면이 비지 않게 이전 데이터를 유지

type ApmSpan = Span<ApmInterval>
const spanKey = (s: ApmSpan) => [s.from, s.to, s.interval]

export function useApmSummary(minutes = NOW_WINDOW_MINUTES) {
  return useQuery({
    queryKey: ['apm', 'summary', minutes],
    queryFn: ({ signal }) => fetchApmSummary(minutes, signal),
    refetchInterval: REFRESH_MS,
  })
}

export function useApmSeries(span: ApmSpan, service: string) {
  return useQuery({
    queryKey: ['apm', 'series', service, ...spanKey(span)],
    queryFn: ({ signal }) => fetchApmSeries(span, service, signal),
    refetchInterval: REFRESH_MS * 2,
    placeholderData: keep,
  })
}

export function useEndpoints(span: ApmSpan, service: string, limit = 10) {
  return useQuery({
    queryKey: ['apm', 'endpoints', service, limit, ...spanKey(span)],
    queryFn: ({ signal }) => fetchEndpoints(span, service, limit, signal),
    refetchInterval: REFRESH_MS * 2,
    placeholderData: keep,
  })
}

export function useErrorGroups(span: ApmSpan, service: string) {
  return useQuery({
    queryKey: ['apm', 'errors', service, ...spanKey(span)],
    queryFn: ({ signal }) => fetchErrorGroups(span, service, signal),
    refetchInterval: REFRESH_MS * 2,
    placeholderData: keep,
  })
}

// JVM 지표는 서비스 하나를 골랐을 때만 의미가 있다.
export function useJvm(span: ApmSpan, service: string) {
  return useQuery({
    queryKey: ['apm', 'jvm', service, ...spanKey(span)],
    queryFn: ({ signal }) => fetchJvm(span, service, signal),
    refetchInterval: REFRESH_MS * 2,
    placeholderData: keep,
    enabled: service !== '',
  })
}
