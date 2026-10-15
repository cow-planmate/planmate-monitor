// Package timeseries 는 시간 구간을 버킷으로 나누고, 데이터가 없는 버킷을 0으로 채운다.
//
// 버킷 경계는 표시 시간대(KST) 기준이다. "하루"는 KST 자정부터 자정까지다.
package timeseries

import (
	"errors"
	"fmt"
	"time"
)

type Interval string

const (
	Minute5  Interval = "5m"
	Minute15 Interval = "15m"
	Hour     Interval = "hour"
	Day      Interval = "day"
	Week     Interval = "week"
	Month    Interval = "month"
)

// MaxBuckets 는 한 요청이 만들 수 있는 버킷 수 상한이다.
// "1년을 1시간 간격으로" 같은 요청이 DB 와 브라우저를 함께 무겁게 만드는 것을 막는다.
const MaxBuckets = 1000

var ErrInvalidRange = errors.New("invalid range")

func ParseInterval(s string) (Interval, error) {
	switch iv := Interval(s); iv {
	case Minute5, Minute15, Hour, Day, Week, Month:
		return iv, nil
	}
	return "", fmt.Errorf("%w: interval 은 5m, 15m, hour, day, week, month 중 하나여야 한다", ErrInvalidRange)
}

// Truncate 는 t 를 loc 기준으로 버킷 시작 시각으로 내린다.
// 주의 시작은 월요일이다 — Postgres date_trunc('week') 와 같다.
func (iv Interval) Truncate(t time.Time, loc *time.Location) time.Time {
	l := t.In(loc)
	switch iv {
	case Minute5, Minute15:
		step := iv.step()
		return time.Date(l.Year(), l.Month(), l.Day(), l.Hour(), l.Minute()-l.Minute()%int(step/time.Minute), 0, 0, loc)
	case Hour:
		return time.Date(l.Year(), l.Month(), l.Day(), l.Hour(), 0, 0, 0, loc)
	case Day:
		return time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, loc)
	case Week:
		d := time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, loc)
		offset := (int(d.Weekday()) + 6) % 7 // 월=0 … 일=6
		return d.AddDate(0, 0, -offset)
	case Month:
		return time.Date(l.Year(), l.Month(), 1, 0, 0, 0, 0, loc)
	}
	return l
}

// Next 는 버킷 시작 시각 t 다음 버킷의 시작 시각이다.
func (iv Interval) Next(t time.Time) time.Time {
	switch iv {
	case Minute5, Minute15:
		return t.Add(iv.step())
	case Hour:
		return t.Add(time.Hour)
	case Day:
		return t.AddDate(0, 0, 1)
	case Week:
		return t.AddDate(0, 0, 7)
	case Month:
		return t.AddDate(0, 1, 0)
	}
	return t
}

func (iv Interval) step() time.Duration {
	if iv == Minute5 {
		return 5 * time.Minute
	}
	return 15 * time.Minute
}

// Range 는 [From, To) 구간과 버킷 간격이다. From 은 버킷 경계로 내려져 있다.
type Range struct {
	From     time.Time
	To       time.Time
	Interval Interval
	Location *time.Location
}

// NewRange 는 버킷 수가 MaxBuckets 이하인 구간을 만든다.
func NewRange(from, to time.Time, iv Interval, loc *time.Location) (Range, error) {
	return NewRangeMax(from, to, iv, loc, MaxBuckets)
}

// NewRangeMax 는 버킷 수 상한을 직접 정한다. APM 처럼 버킷마다 서비스 수만큼 행이 생기는 경우에 쓴다.
func NewRangeMax(from, to time.Time, iv Interval, loc *time.Location, maxBuckets int) (Range, error) {
	if !from.Before(to) {
		return Range{}, fmt.Errorf("%w: from 은 to 보다 앞이어야 한다", ErrInvalidRange)
	}
	r := Range{From: iv.Truncate(from, loc), To: to.In(loc), Interval: iv, Location: loc}
	n := 0
	for t := r.From; t.Before(r.To); t = iv.Next(t) {
		if n++; n > maxBuckets {
			return Range{}, fmt.Errorf("%w: 버킷이 %d개를 넘는다. 기간을 줄이거나 간격을 늘려야 한다", ErrInvalidRange, maxBuckets)
		}
	}
	return r, nil
}

// Buckets 는 구간 안의 모든 버킷 시작 시각이다.
func (r Range) Buckets() []time.Time {
	var out []time.Time
	for t := r.From; t.Before(r.To); t = r.Interval.Next(t) {
		out = append(out, t)
	}
	return out
}

type Point struct {
	T     time.Time `json:"t"`
	Value int64     `json:"value"`
}

// Fill 은 모든 버킷에 대해 점을 만든다. counts 에 없는 버킷은 0 이다.
// counts 의 키는 버킷 시작 시각이며, 시간대와 무관하게 같은 시각(instant)이면 같은 버킷으로 본다.
func Fill(r Range, counts map[time.Time]int64) []Point {
	byUnix := make(map[int64]int64, len(counts))
	for t, v := range counts {
		byUnix[t.Unix()] += v
	}
	buckets := r.Buckets()
	out := make([]Point, len(buckets))
	for i, t := range buckets {
		out[i] = Point{T: t.UTC(), Value: byUnix[t.Unix()]}
	}
	return out
}
