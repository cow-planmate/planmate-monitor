import { useApmSummary } from '../queries'
import { shortServiceName, useServiceFilter } from '../serviceFilter'

// 서비스 선택 탭. 목록은 최근 24시간 동안 요청이 있었던 서비스다.
export function ServiceTabs() {
  const { service, setService } = useServiceFilter()
  const { data } = useApmSummary(24 * 60)
  const names = (data?.services ?? []).map((s) => s.service)
  // URL 로 들어온 서비스가 목록에 없어도(최근 요청 없음) 선택 상태는 보이게 한다.
  if (service && !names.includes(service)) names.push(service)

  const tab = (value: string, label: string) => (
    <button
      key={value || 'all'}
      type="button"
      onClick={() => setService(value)}
      aria-pressed={service === value}
      className={`rounded-md px-3 py-1.5 text-sm ${
        service === value ? 'bg-surface-2 font-medium text-fg' : 'text-muted hover:text-fg'
      }`}
    >
      {label}
    </button>
  )

  return (
    <div role="group" aria-label="서비스 선택" className="flex flex-wrap gap-1 rounded-lg border border-line bg-surface p-1">
      {tab('', '전체')}
      {names.map((n) => tab(n, shortServiceName(n)))}
    </div>
  )
}
