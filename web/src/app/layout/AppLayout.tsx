import { NavLink, Outlet, useLocation } from 'react-router-dom'
import { useIsFetching, useQueryClient } from '@tanstack/react-query'
import { SourceStatusList } from '../../features/health/components/SourceStatusList'
import { RANGES, useRangeFilter } from '../../shared/filters/range'
import { formatClock } from '../../shared/lib/format'
import { useMeta } from '../../features/meta/queries'

const NAV = [
  { to: '/', label: '개요', end: true },
  { to: '/users', label: '사용자' },
  { to: '/performance', label: '성능' },
  { to: '/errors', label: '에러' },
]

export function AppLayout() {
  const { search } = useLocation()
  const meta = useMeta()

  return (
    <div className="flex min-h-screen">
      <aside className="sticky top-0 hidden h-screen w-52 shrink-0 flex-col border-r border-line bg-surface md:flex">
        <div className="px-5 py-5">
          <p className="text-sm font-bold">PlanMate</p>
          <p className="text-xs text-muted">Monitor{meta.data ? ` · ${meta.data.environment}` : ''}</p>
        </div>
        <nav className="flex-1 space-y-0.5 px-3">
          {NAV.map((n) => (
            <NavLink
              key={n.to}
              to={{ pathname: n.to, search }} // 기간 필터를 화면 이동에도 유지한다
              end={n.end}
              className={({ isActive }) =>
                `block rounded-md px-3 py-2 text-sm ${isActive ? 'bg-surface-2 font-medium text-fg' : 'text-muted hover:text-fg'}`
              }
            >
              {n.label}
            </NavLink>
          ))}
        </nav>
        <div className="border-t border-line px-5 py-4">
          <p className="mb-2 text-xs font-medium text-muted">데이터 소스</p>
          <SourceStatusList />
        </div>
      </aside>

      <div className="min-w-0 flex-1">
        <TopBar />
        <main className="mx-auto max-w-7xl space-y-4 p-4 md:p-6">
          <Outlet />
        </main>
      </div>
    </div>
  )
}

// 화면에 있는 데이터 중 가장 최근에 받아 온 시각. 실패한 조회는 dataUpdatedAt 을 바꾸지 않으므로
// 이 값은 "실제로 데이터가 갱신된 시각" 이다. 조회가 시작·끝날 때마다(useIsFetching) 다시 계산된다.
function useLastUpdated(): number {
  const client = useQueryClient()
  useIsFetching()
  return client
    .getQueryCache()
    .getAll()
    .reduce((latest, q) => Math.max(latest, q.state.dataUpdatedAt), 0)
}

function TopBar() {
  const { key, setKey } = useRangeFilter()
  const fetching = useIsFetching()
  const updatedAt = useLastUpdated()

  return (
    <header className="sticky top-0 z-10 flex flex-wrap items-center justify-end gap-3 border-b border-line bg-bg/90 px-4 py-3 backdrop-blur md:px-6">
      <span className="mr-auto text-xs text-muted md:hidden">PlanMate Monitor</span>
      <label className="flex items-center gap-2 text-xs text-muted">
        추이 기간
        <select
          value={key}
          onChange={(e) => setKey(e.target.value as typeof key)}
          className="rounded-md border border-line bg-surface px-2 py-1 text-sm text-fg"
        >
          {RANGES.map((r) => (
            <option key={r.key} value={r.key}>
              최근 {r.label}
            </option>
          ))}
        </select>
      </label>
      <span className="tabular text-xs text-muted" aria-live="polite">
        {fetching > 0
          ? '갱신 중…'
          : updatedAt > 0
            ? `마지막 갱신 ${formatClock(new Date(updatedAt))} · 30초마다`
            : '30초마다 갱신'}
      </span>
    </header>
  )
}
