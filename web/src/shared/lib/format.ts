// 숫자·시간 표시. 시간은 서버가 UTC 로 주고, 화면은 항상 KST 로 보여준다.

export const DISPLAY_TZ = 'Asia/Seoul'

const intFmt = new Intl.NumberFormat('ko-KR')

export function formatInt(n: number | null | undefined): string {
  return n == null ? '—' : intFmt.format(n)
}

export function formatPercent(ratio: number | null | undefined, digits = 1): string {
  return ratio == null ? '—' : `${(ratio * 100).toFixed(digits)}%`
}

export function formatMs(ms: number | null | undefined): string {
  if (ms == null) return '—'
  return ms >= 1000 ? `${(ms / 1000).toFixed(2)}s` : `${Math.round(ms)}ms`
}

export function formatRate(perMin: number | null | undefined): string {
  return perMin == null ? '—' : `${perMin.toFixed(perMin < 10 ? 1 : 0)} rpm`
}

const dateFmt = new Intl.DateTimeFormat('ko-KR', { timeZone: DISPLAY_TZ, month: 'numeric', day: 'numeric' })
const hourFmt = new Intl.DateTimeFormat('ko-KR', { timeZone: DISPLAY_TZ, hour: '2-digit', hour12: false })
const timeFmt = new Intl.DateTimeFormat('ko-KR', {
  timeZone: DISPLAY_TZ,
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit',
  hour12: false,
})

const hmFmt = new Intl.DateTimeFormat('ko-KR', { timeZone: DISPLAY_TZ, hour: '2-digit', minute: '2-digit', hour12: false })

export function formatBucket(iso: string, interval: string): string {
  const d = new Date(iso)
  switch (interval) {
    case '5m':
    case '15m':
      return hmFmt.format(d)
    case 'hour':
      return `${dateFmt.format(d)} ${hourFmt.format(d)}시`
    default:
      return dateFmt.format(d)
  }
}

const dateTimeFmt = new Intl.DateTimeFormat('ko-KR', {
  timeZone: DISPLAY_TZ,
  month: 'numeric',
  day: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
  hour12: false,
})

export function formatDateTime(iso: string): string {
  return dateTimeFmt.format(new Date(iso))
}

const rtf = new Intl.RelativeTimeFormat('ko-KR', { numeric: 'auto' })

// "3분 전", "2시간 전" 처럼 지금 기준 상대 시각.
export function formatAgo(iso: string, now = Date.now()): string {
  const sec = Math.round((new Date(iso).getTime() - now) / 1000)
  const abs = Math.abs(sec)
  if (abs < 60) return rtf.format(sec, 'second')
  if (abs < 3600) return rtf.format(Math.round(sec / 60), 'minute')
  if (abs < 86400) return rtf.format(Math.round(sec / 3600), 'hour')
  return rtf.format(Math.round(sec / 86400), 'day')
}

export function formatClock(iso: string | Date): string {
  return timeFmt.format(typeof iso === 'string' ? new Date(iso) : iso)
}
