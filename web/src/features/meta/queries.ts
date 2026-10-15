import { useQuery } from '@tanstack/react-query'
import { fetchMeta } from './api'

// 서버 설정이라 바뀌지 않는다. 한 번만 받는다.
export function useMeta() {
  return useQuery({
    queryKey: ['meta'],
    queryFn: ({ signal }) => fetchMeta(signal),
    staleTime: Infinity,
  })
}
