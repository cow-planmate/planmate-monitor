// 서버 internal/stats 의 JSON 응답과 1:1 로 맞춘다.

export interface Count {
  total: number
  today: number
}

export interface Overview {
  generatedAt: string
  users: Count | null
  plans: Count | null
  feedPosts: Count | null
  communityPosts: Count | null
  activeUsersToday: number | null
  errors?: Record<string, string> // 출처(planmate, community) → 사유
}

export type Metric =
  | 'signups'
  | 'plans'
  | 'plan_joins'
  | 'collaboration_requests'
  | 'feed_posts'
  | 'community_posts'
  | 'feed_comments'
  | 'community_comments'
  | 'feed_reactions'
  | 'community_reactions'

export interface PlanSummary {
  plans: number
  sharedPlans: number
  avgMembers: number
  collaborationByStatus: Partial<Record<'PENDING' | 'ACCEPTED' | 'REJECTED', number>>
}

export interface Feedback {
  id: number
  content: string
  createdAt: string
}

export interface Point {
  t: string
  value: number
}

export interface TimeseriesResponse {
  metric: Metric
  interval: string
  points: Point[]
}
