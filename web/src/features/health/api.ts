import { apiGet } from '../../shared/api/client'
import type { HealthReport } from './types'

export const fetchHealth = (signal?: AbortSignal) => apiGet<HealthReport>('/api/health', undefined, signal)
