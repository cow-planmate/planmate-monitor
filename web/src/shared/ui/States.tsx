// 패널 단위 로딩·에러 표시. 한 지표가 실패해도 나머지 화면은 그대로 보이게 한다.

export function ErrorState({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <div className="flex h-full min-h-24 flex-col items-center justify-center gap-2 text-center">
      <p className="text-sm text-danger">{message}</p>
      {onRetry && (
        <button
          type="button"
          onClick={onRetry}
          className="rounded border border-line px-2.5 py-1 text-xs text-muted hover:text-fg"
        >
          다시 시도
        </button>
      )}
    </div>
  )
}

export function Skeleton({ className = 'h-48' }: { className?: string }) {
  return <div className={`animate-pulse rounded bg-surface-2 ${className}`} />
}

export function StatusDot({ ok }: { ok: boolean | undefined }) {
  const color = ok === undefined ? 'bg-muted' : ok ? 'bg-ok' : 'bg-danger'
  return <span className={`inline-block size-2 shrink-0 rounded-full ${color}`} aria-hidden />
}

export function Banner({ children }: { children: React.ReactNode }) {
  return (
    <div role="status" className="rounded-md border border-warn/40 bg-warn/10 px-3 py-2 text-sm text-warn">
      {children}
    </div>
  )
}
