package main

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

type Limit struct {
	limit    int
	lastReqs []time.Time
}

func NewLimit(limit int) *Limit {
	return &Limit{
		limit:    limit,
		lastReqs: make([]time.Time, limit),
	}
}

func (l *Limit) AddReq(t time.Time) {
	if len(l.lastReqs) == l.limit {
		for i := 1; i < len(l.lastReqs); i++ {
			l.lastReqs[i-1] = l.lastReqs[i]
		}
		l.lastReqs[len(l.lastReqs)-1] = t
		return
	}

	l.lastReqs = append(l.lastReqs, t)
}

type Throttler struct {
	limit int

	mx sync.RWMutex
	m  map[string]*Limit
}

func NewThrottler(limit int) *Throttler {
	return &Throttler{
		limit: limit,
		mx:    sync.RWMutex{},
		m:     make(map[string]*Limit),
	}
}

func (t *Throttler) Allow(userID string) bool {
	if _, ok := t.m[userID]; !ok {
		t.m[userID] = NewLimit(t.limit)
	}

	limit := t.m[userID]

	currentTime := time.Now()

	count := 0

	for _, tt := range limit.lastReqs {
		if tt.After(currentTime.Add(-1 * time.Minute)) {
			count++
		}
	}

	if count < t.limit {
		return true
	}

	return false
}

var (
	LimitedErr = fmt.Errorf("limited")
)

func (t *Throttler) Try(userID string) error {
	if allowed := t.Allow(userID); !allowed {
		return LimitedErr
	}

	t.mx.RLock()
	t.m[userID].AddReq(time.Now())
	t.mx.RUnlock()

	return nil
}

func main() {
	throttler := NewThrottler(2)

	http.HandleFunc("/limited/", func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("User-ID")

		if err := throttler.Try(userID); err != nil {
			w.WriteHeader(http.StatusTooManyRequests)
		}
	})

	http.ListenAndServe(":8081", nil)
}
