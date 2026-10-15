import { KpiCard } from '../../../shared/ui/KpiCard'
import { formatInt, formatPercent } from '../../../shared/lib/format'
import { useOverview, usePlanSummary } from '../queries'

// 사용자 화면 KPI. 회원 현황은 개요와 같은 출처이고, 일정 공동 작업 현황이 더해진다.
export function UserKpis() {
  const overview = useOverview()
  const plans = usePlanSummary()
  const p = plans.data
  const collab = p?.collaborationByStatus ?? {}
  const decided = (collab.ACCEPTED ?? 0) + (collab.REJECTED ?? 0)
  const acceptRate = decided > 0 ? (collab.ACCEPTED ?? 0) / decided : null

  return (
    <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
      <KpiCard
        label="회원"
        value={formatInt(overview.data?.users?.total)}
        sub={overview.data?.users ? `오늘 +${formatInt(overview.data.users.today)}` : undefined}
        loading={overview.isPending}
        unavailable={overview.isError || (overview.data && !overview.data.users)}
      />
      <KpiCard
        label="공동 작업 일정"
        value={p ? formatPercent(p.plans > 0 ? p.sharedPlans / p.plans : null) : '—'}
        sub={p ? `${formatInt(p.sharedPlans)} / ${formatInt(p.plans)}개 · 멤버 2명 이상` : undefined}
        loading={plans.isPending}
        unavailable={plans.isError}
      />
      <KpiCard
        label="일정당 평균 멤버"
        value={p ? `${p.avgMembers.toFixed(2)}명` : '—'}
        sub="삭제된 일정 제외"
        loading={plans.isPending}
        unavailable={plans.isError}
      />
      <KpiCard
        label="협업 요청 수락률"
        value={formatPercent(acceptRate)}
        sub={p ? `대기 ${formatInt(collab.PENDING ?? 0)}건 · 거절 ${formatInt(collab.REJECTED ?? 0)}건` : undefined}
        loading={plans.isPending}
        unavailable={plans.isError}
      />
    </div>
  )
}
