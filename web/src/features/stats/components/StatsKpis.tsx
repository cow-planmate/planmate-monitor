import { KpiCard } from '../../../shared/ui/KpiCard'
import { Banner } from '../../../shared/ui/States'
import { formatInt } from '../../../shared/lib/format'
import { useOverview } from '../queries'
import type { Count } from '../types'

const todaySub = (c: Count | null | undefined) => (c ? `오늘 +${formatInt(c.today)}` : undefined)

export function StatsKpis() {
  const { data, isPending, isError, error } = useOverview()
  const loading = isPending
  const failed = isError || undefined

  return (
    <div className="space-y-3">
      {isError && <Banner>사용자 통계를 불러오지 못했다: {error.message}</Banner>}
      {data?.errors && (
        <Banner>
          일부 지표를 불러오지 못했다 — {Object.values(data.errors).join(', ')}. 나머지 지표는 정상이다.
        </Banner>
      )}
      <div className="grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-5">
        <KpiCard
          label="회원"
          value={formatInt(data?.users?.total)}
          sub={todaySub(data?.users)}
          loading={loading}
          unavailable={failed || (data && !data.users)}
          to="/users"
        />
        <KpiCard
          label="일정"
          value={formatInt(data?.plans?.total)}
          sub={todaySub(data?.plans)}
          loading={loading}
          unavailable={failed || (data && !data.plans)}
          to="/users"
        />
        <KpiCard
          label="피드"
          value={formatInt(data?.feedPosts?.total)}
          sub={todaySub(data?.feedPosts)}
          loading={loading}
          unavailable={failed || (data && !data.feedPosts)}
          to="/users"
        />
        <KpiCard
          label="커뮤니티 글"
          value={formatInt(data?.communityPosts?.total)}
          sub={todaySub(data?.communityPosts)}
          loading={loading}
          unavailable={failed || (data && !data.communityPosts)}
          to="/users"
        />
        <KpiCard
          label="오늘 활동 사용자"
          value={formatInt(data?.activeUsersToday)}
          sub="일정·글·댓글 작성 기준"
          loading={loading}
          unavailable={failed || (data && data.activeUsersToday == null)}
          to="/users"
        />
      </div>
    </div>
  )
}
