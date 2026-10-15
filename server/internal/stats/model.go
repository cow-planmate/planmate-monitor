// Package stats 는 서비스 DB 에서 사용자·콘텐츠 통계를 뽑는다.
//
// 지표 정의 (2026-10-15 결정):
//   - 회원: 현재 남아 있는 회원. ADMIN 은 제외한다. 탈퇴는 하드 삭제라 행이 이미 없다.
//     그래서 가입 추이는 "그때 가입해서 아직 남아 있는 회원" 기준이다.
//   - 일정/글 현황(Total, Today): 현재 존재하는 것. 소프트 삭제(deleted_at)는 제외한다.
//   - 일정/글 추이(시계열): 생성 건수. 나중에 소프트 삭제된 것도 그때 만들어진 것이므로 포함한다.
//   - 활동 사용자: 기간 안에 일정을 만들거나 수정했거나, 글·댓글을 쓴 사람(ADMIN 제외).
//     둘러보기만 한 사람은 DB 에 흔적이 없어 셀 수 없다.
package stats

import (
	"errors"
	"time"
)

type Count struct {
	Total int64 `json:"total"`
	Today int64 `json:"today"`
}

// Overview 는 개요 화면의 KPI 다. 출처 DB 하나가 실패해도 나머지는 채워서 돌려준다.
// 실패한 항목은 null 이고, Errors 에 출처별 사유가 들어간다.
type Overview struct {
	GeneratedAt      time.Time         `json:"generatedAt"`
	Users            *Count            `json:"users"`
	Plans            *Count            `json:"plans"`
	FeedPosts        *Count            `json:"feedPosts"`
	CommunityPosts   *Count            `json:"communityPosts"`
	ActiveUsersToday *int64            `json:"activeUsersToday"`
	Errors           map[string]string `json:"errors,omitempty"`
}

type Metric string

// 시계열 지표. 출처 DB 별로 나뉜다(서비스가 어느 store 에 물을지 정한다).
const (
	// planmate DB
	MetricSignups               Metric = "signups"
	MetricPlans                 Metric = "plans"
	MetricPlanJoins             Metric = "plan_joins" // 공동 편집자(EDITOR) 합류
	MetricCollaborationRequests Metric = "collaboration_requests"

	// community DB
	MetricFeedPosts          Metric = "feed_posts"
	MetricCommunityPosts     Metric = "community_posts"
	MetricFeedComments       Metric = "feed_comments"
	MetricCommunityComments  Metric = "community_comments"
	MetricFeedReactions      Metric = "feed_reactions"
	MetricCommunityReactions Metric = "community_reactions"
)

// PlanSummary 는 일정 공동 작업 현황이다(소프트 삭제된 일정 제외).
type PlanSummary struct {
	Plans       int64   `json:"plans"`
	SharedPlans int64   `json:"sharedPlans"` // 멤버가 2명 이상인 일정
	AvgMembers  float64 `json:"avgMembers"`
	// 협업 요청 상태별 누적 건수: PENDING, ACCEPTED, REJECTED
	CollaborationByStatus map[string]int64 `json:"collaborationByStatus"`
}

// Feedback 은 베타 피드백 한 건이다. 사용자 원문이라 관리자 화면에서만 보인다.
type Feedback struct {
	ID        int64     `json:"id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

const MaxFeedbackLimit = 100

var (
	ErrUnknownMetric   = errors.New("unknown metric")
	ErrInvalidInterval = errors.New("invalid interval")
)

// 출처 이름. Overview.Errors 의 키이자 로그 필드다.
const (
	SourcePlanmate  = "planmate"
	SourceCommunity = "community"
)
