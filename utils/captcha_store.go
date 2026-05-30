package utils

import (
	"container/list"
	"strings"
	"sync"
	"time"

	"github.com/mojocn/base64Captcha"
)

type boundedCaptchaStoreValue struct {
	timestamp time.Time
	id        string
}

type boundedCaptchaStore struct {
	mu         sync.Mutex
	digitsByID map[string]string
	elemByID   map[string]*list.Element
	byTime     *list.List
	maxEntries int
	expiration time.Duration
}

func NewBoundedCaptchaStore(maxEntries int, expiration time.Duration) base64Captcha.Store {
	s := &boundedCaptchaStore{
		digitsByID: make(map[string]string),
		elemByID:   make(map[string]*list.Element),
		byTime:     list.New(),
		maxEntries: maxEntries,
		expiration: expiration,
	}
	return s
}

func (s *boundedCaptchaStore) Set(id string, value string) error {
	if id == "" {
		return nil
	}

	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	s.collectExpiredLocked(now)

	if e := s.elemByID[id]; e != nil {
		s.byTime.Remove(e)
		delete(s.elemByID, id)
	}

	switch {
	case s.maxEntries > 0 && len(s.digitsByID) >= s.maxEntries:
		s.evictOldestLocked(1)
	default:
	}

	s.digitsByID[id] = value
	s.elemByID[id] = s.byTime.PushBack(boundedCaptchaStoreValue{
		timestamp: now,
		id:        id,
	})
	return nil
}

func (s *boundedCaptchaStore) Get(id string, clear bool) string {
	if id == "" {
		return ""
	}

	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	s.collectExpiredLocked(now)

	v, ok := s.digitsByID[id]
	if !ok {
		return ""
	}

	switch {
	case clear:
		delete(s.digitsByID, id)
		if e := s.elemByID[id]; e != nil {
			s.byTime.Remove(e)
		}
		delete(s.elemByID, id)
	default:
	}

	return v
}

func (s *boundedCaptchaStore) Verify(id, answer string, clear bool) bool {
	if id == "" || answer == "" {
		return false
	}
	v := s.Get(id, clear)
	return strings.EqualFold(v, answer)
}

func (s *boundedCaptchaStore) collectExpiredLocked(now time.Time) {
	for e := s.byTime.Front(); e != nil; {
		ev, ok := e.Value.(boundedCaptchaStoreValue)
		if !ok {
			next := e.Next()
			s.byTime.Remove(e)
			e = next
			continue
		}

		switch {
		case ev.timestamp.Add(s.expiration).Before(now):
			delete(s.digitsByID, ev.id)
			delete(s.elemByID, ev.id)
			next := e.Next()
			s.byTime.Remove(e)
			e = next
		default:
			return
		}
	}
}

func (s *boundedCaptchaStore) evictOldestLocked(n int) {
	for i := 0; i < n; i++ {
		e := s.byTime.Front()
		if e == nil {
			return
		}
		ev, ok := e.Value.(boundedCaptchaStoreValue)
		if ok {
			delete(s.digitsByID, ev.id)
			delete(s.elemByID, ev.id)
		}
		s.byTime.Remove(e)
	}
}
