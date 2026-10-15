import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'

export type Tone = 'neutral' | 'ok' | 'warn' | 'danger'

const toneClass: Record<Tone, string> = {
  neutral: 'text-fg',
  ok: 'text-ok',
  warn: 'text-warn',
  danger: 'text-danger',
}

interface KpiCardProps {
  label: string
  value: ReactNode
  sub?: ReactNode
  tone?: Tone
  loading?: boolean
  unavailable?: boolean // 출처 조회 실패로 값이 없음
  to?: string // 누르면 상세 화면으로
}

export function KpiCard({ label, value, sub, tone = 'neutral', loading, unavailable, to }: KpiCardProps) {
  const body = (
    <div className="h-full rounded-lg border border-line bg-surface px-4 py-3 transition-colors hover:border-muted/60">
      <p className="text-xs text-muted">{label}</p>
      {loading ? (
        <div className="mt-2 h-7 w-20 animate-pulse rounded bg-surface-2" />
      ) : unavailable ? (
        <p className="mt-1 text-2xl font-semibold text-muted">—</p>
      ) : (
        <p className={`tabular mt-1 text-2xl font-semibold ${toneClass[tone]}`}>{value}</p>
      )}
      <p className="tabular mt-1 min-h-4 text-xs text-muted">{loading ? '' : unavailable ? '조회 실패' : sub}</p>
    </div>
  )
  return to ? (
    <Link to={to} className="block focus:outline-none focus-visible:ring-2 focus-visible:ring-accent rounded-lg">
      {body}
    </Link>
  ) : (
    body
  )
}
