import { StatusDot } from '../../../shared/ui/States'
import { useHealth } from '../queries'

const LABELS: Record<string, string> = {
  'planmate-db': 'planmate DB',
  'community-db': 'community DB',
  'notification-db': 'notification DB',
  elasticsearch: 'Elasticsearch',
}

// 데이터 소스 연결 상태. 화면에 값이 안 나올 때 원인이 소스 쪽인지 바로 보이게 한다.
export function SourceStatusList() {
  const { data, isError } = useHealth()

  if (isError) {
    return <p className="text-xs text-danger">monitor 서버에 연결할 수 없다</p>
  }
  return (
    <ul className="space-y-1.5">
      {(data?.sources ?? []).map((s) => (
        <li key={s.name} className="flex items-center gap-2 text-xs" title={s.error ?? `${s.latencyMs}ms`}>
          <StatusDot ok={s.ok} />
          <span className={s.ok ? 'text-muted' : 'text-danger'}>{LABELS[s.name] ?? s.name}</span>
          {!s.ok && <span className="ml-auto text-danger">{s.error}</span>}
        </li>
      ))}
    </ul>
  )
}
