import { useQuery } from '@tanstack/react-query'
import { fetchHealth } from './api'

export function useHealth() {
  return useQuery({
    queryKey: ['health'],
    queryFn: ({ signal }) => fetchHealth(signal),
    refetchInterval: 15_000,
  })
}
