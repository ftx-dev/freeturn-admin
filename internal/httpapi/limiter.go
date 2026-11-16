package httpapi

import (
	"sync"
	"time"
)

// limiter - не больше n попыток с ключа (IP, логин) за окно. Счётчики в памяти процесса.
// shortcut: в памяти одного процесса; при нескольких репликах перенести в БД.
type limiter struct {
	mu     sync.Mutex
	n      int
	window time.Duration
	hits   map[string][]time.Time
}

func newLimiter(n int, window time.Duration) *limiter {
	return &limiter{n: n, window: window, hits: map[string][]time.Time{}}
}

// Allow засчитывает попытку и говорит, укладывается ли она в лимит.
func (l *limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	keep := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if now.Sub(t) < l.window {
			keep = append(keep, t)
		}
	}
	if len(keep) >= l.n {
		l.hits[key] = keep
		return false
	}
	l.hits[key] = append(keep, now)
	if len(l.hits) > 10000 { // защита памяти от перебора ключей
		l.hits = map[string][]time.Time{key: l.hits[key]}
	}
	return true
}

// Reset - после успешного входа счётчик ключа обнуляется.
func (l *limiter) Reset(key string) {
	l.mu.Lock()
	delete(l.hits, key)
	l.mu.Unlock()
}
