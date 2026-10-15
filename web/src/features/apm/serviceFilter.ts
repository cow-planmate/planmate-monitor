import { useSearchParams } from 'react-router-dom'

// 서비스 필터. URL(?service=planmate-gateway)에 저장해서 링크로 공유할 수 있다. 빈 값은 전체다.
export function useServiceFilter() {
  const [params, setParams] = useSearchParams()
  const service = params.get('service') ?? ''

  const setService = (next: string) =>
    setParams(
      (prev) => {
        const p = new URLSearchParams(prev)
        if (next) p.set('service', next)
        else p.delete('service')
        return p
      },
      { replace: true },
    )

  return { service, setService }
}

export const shortServiceName = (s: string) => s.replace(/^planmate-/, '')
