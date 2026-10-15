import { Panel } from '../../../shared/ui/Panel'
import { ErrorState, Skeleton } from '../../../shared/ui/States'
import { formatAgo, formatDateTime } from '../../../shared/lib/format'
import { useFeedback } from '../queries'

// 최근 베타 피드백. 사용자 원문이라 관리자 화면에서만 보인다.
export function FeedbackList() {
  const { data, isPending, error, refetch } = useFeedback(20)

  return (
    <Panel title="최근 베타 피드백" subtitle="최신 20건">
      {error ? (
        <ErrorState message={error.message} onRetry={() => void refetch()} />
      ) : isPending ? (
        <Skeleton className="h-64" />
      ) : data.items.length === 0 ? (
        <p className="py-8 text-center text-sm text-muted">아직 피드백이 없다</p>
      ) : (
        <ul className="max-h-[420px] divide-y divide-line overflow-y-auto">
          {data.items.map((f) => (
            <li key={f.id} className="py-2.5">
              <p className="text-sm whitespace-pre-wrap break-words">{f.content}</p>
              <p className="mt-1 text-xs text-muted" title={formatDateTime(f.createdAt)}>
                {formatAgo(f.createdAt)}
              </p>
            </li>
          ))}
        </ul>
      )}
    </Panel>
  )
}
