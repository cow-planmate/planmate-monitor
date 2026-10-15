import { UserKpis } from '../features/stats/components/UserKpis'
import { MetricsChart, type MetricLine } from '../features/stats/components/MetricsChart'
import { FeedbackList } from '../features/stats/components/FeedbackList'
import { useRangeFilter } from '../shared/filters/range'

const SIGNUPS: MetricLine[] = [{ metric: 'signups', name: '가입', type: 'bar' }]

const PLANS: MetricLine[] = [
  { metric: 'plans', name: '일정 생성', type: 'bar' },
  { metric: 'plan_joins', name: '공동 편집자 합류', type: 'line' },
  { metric: 'collaboration_requests', name: '협업 요청', type: 'line' },
]

const CONTENT: MetricLine[] = [
  { metric: 'feed_posts', name: '피드 글', type: 'bar', stack: 'posts' },
  { metric: 'community_posts', name: '커뮤니티 글', type: 'bar', stack: 'posts' },
  { metric: 'feed_comments', name: '피드 댓글', type: 'line', axis: 1 },
  { metric: 'community_comments', name: '커뮤니티 댓글', type: 'line', axis: 1 },
]

const REACTIONS: MetricLine[] = [
  { metric: 'feed_reactions', name: '피드', type: 'bar', stack: 'reactions' },
  { metric: 'community_reactions', name: '커뮤니티', type: 'bar', stack: 'reactions' },
]

// 사용자: 가입·일정·콘텐츠 활동 추이와 피드백. 화면 배치만 하고 데이터는 stats 기능이 가져온다.
export function UsersPage() {
  const { range } = useRangeFilter()
  const span = range.stats

  return (
    <>
      <h1 className="text-lg font-semibold">사용자</h1>
      <UserKpis />
      <div className="grid gap-4 lg:grid-cols-2">
        <MetricsChart
          title="가입 추이"
          subtitle={`최근 ${span.label} · 현재 남아 있는 회원 기준(ADMIN 제외)`}
          lines={SIGNUPS}
          span={span}
        />
        <MetricsChart title="일정 · 공동 작업" lines={PLANS} span={span} />
        <MetricsChart
          title="콘텐츠 작성"
          subtitle={`최근 ${span.label} · 막대는 글(왼쪽 축), 선은 댓글(오른쪽 축)`}
          lines={CONTENT}
          span={span}
        />
        <MetricsChart
          title="반응(좋아요·싫어요)"
          subtitle={`최근 ${span.label} · 취소된 반응은 기록이 지워져 빠진다`}
          lines={REACTIONS}
          span={span}
        />
      </div>
      <FeedbackList />
    </>
  )
}
