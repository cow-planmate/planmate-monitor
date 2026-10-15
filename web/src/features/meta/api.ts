import { apiGet } from '../../shared/api/client'
import type { Meta } from './types'

export const fetchMeta = (signal?: AbortSignal) => apiGet<Meta>('/api/meta', undefined, signal)
