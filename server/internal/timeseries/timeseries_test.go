package timeseries

import (
	"errors"
	"testing"
	"time"
)

var kst = time.FixedZone("KST", 9*60*60)

func TestTruncateDayUsesKSTMidnight(t *testing.T) {
	// UTC 10/14 16:30 = KST 10/15 01:30 → KST 10/15 00:00 버킷이어야 한다.
	in := time.Date(2026, 10, 14, 16, 30, 0, 0, time.UTC)
	got := Day.Truncate(in, kst)
	want := time.Date(2026, 10, 15, 0, 0, 0, 0, kst)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestTruncateWeekStartsMonday(t *testing.T) {
	sunday := time.Date(2026, 10, 18, 12, 0, 0, 0, kst)
	got := Week.Truncate(sunday, kst)
	want := time.Date(2026, 10, 12, 0, 0, 0, 0, kst) // 월요일
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestNewRangeRejectsTooManyBuckets(t *testing.T) {
	from := time.Date(2025, 1, 1, 0, 0, 0, 0, kst)
	to := from.AddDate(1, 0, 0)
	if _, err := NewRange(from, to, Hour, kst); !errors.Is(err, ErrInvalidRange) {
		t.Fatalf("1년을 1시간 간격으로 나누면 거부해야 한다, err=%v", err)
	}
	if _, err := NewRange(from, to, Day, kst); err != nil {
		t.Fatalf("1년을 하루 간격으로 나누는 건 허용해야 한다: %v", err)
	}
}

func TestNewRangeRejectsReversed(t *testing.T) {
	now := time.Now()
	if _, err := NewRange(now, now.Add(-time.Hour), Hour, kst); !errors.Is(err, ErrInvalidRange) {
		t.Fatalf("역순 구간은 거부해야 한다, err=%v", err)
	}
}

func TestFillZeroesMissingBuckets(t *testing.T) {
	from := time.Date(2026, 10, 13, 0, 0, 0, 0, kst)
	r, err := NewRange(from, from.AddDate(0, 0, 3), Day, kst)
	if err != nil {
		t.Fatal(err)
	}
	// 같은 시각을 UTC 로 표현한 키도 같은 버킷으로 합쳐져야 한다.
	counts := map[time.Time]int64{
		from.AddDate(0, 0, 1).UTC(): 5,
	}
	pts := Fill(r, counts)
	if len(pts) != 3 {
		t.Fatalf("버킷 3개여야 한다, got %d", len(pts))
	}
	if pts[0].Value != 0 || pts[1].Value != 5 || pts[2].Value != 0 {
		t.Fatalf("got %+v", pts)
	}
}

func TestTruncateFiveMinutes(t *testing.T) {
	in := time.Date(2026, 10, 15, 13, 47, 31, 0, kst)
	got := Minute5.Truncate(in, kst)
	if want := time.Date(2026, 10, 15, 13, 45, 0, 0, kst); !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if next := Minute5.Next(got); !next.Equal(got.Add(5 * time.Minute)) {
		t.Fatalf("next = %v", next)
	}
}

func TestNewRangeMaxUsesGivenLimit(t *testing.T) {
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, kst)
	if _, err := NewRangeMax(from, from.Add(11*time.Hour), Hour, kst, 10); !errors.Is(err, ErrInvalidRange) {
		t.Fatalf("상한 10개를 넘으면 거부해야 한다, err=%v", err)
	}
}
