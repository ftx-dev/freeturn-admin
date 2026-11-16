package httpapi

import (
	"bufio"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
)

const (
	healthEvery   = 15 * time.Second // не чаще одной проверки сервера за это время
	healthTimeout = 5 * time.Second
	healthDownAt  = 2 // столько неудач подряд - и только тогда сервер «недоступен»
)

// ServerHealth - доступность сервера: TCP до порта SSH и баннер SSH (без входа).
type ServerHealth struct {
	State     string    `json:"state"` // unknown | up | down
	LatencyMs int       `json:"latencyMs,omitempty"`
	Error     string    `json:"error,omitempty"`
	CheckedAt time.Time `json:"checkedAt,omitzero"`

	fails    int
	checking bool
}

type healthCache struct {
	mu sync.Mutex
	m  map[string]*ServerHealth
}

// probeSSH: соединение и строка «SSH-…» от сервера. Вход не нужен - так быстрее и не шумит в журналах сервера.
func probeSSH(host string, port int) (time.Duration, error) {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(strings.Trim(host, "[]"), strconv.Itoa(port)), healthTimeout)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	rtt := time.Since(start)
	conn.SetDeadline(time.Now().Add(healthTimeout))
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return 0, err
	}
	if !strings.HasPrefix(line, "SSH-") {
		return 0, &net.AddrError{Err: "на порту не SSH", Addr: host}
	}
	return rtt, nil
}

// serversHealth отдаёт кэш и в фоне перепроверяет устаревшие записи: ответ мгновенный,
// а при опросе раз в 15 с статус всегда свежий.
func (a *API) serversHealth(ctx *fasthttp.RequestCtx) {
	list, err := a.Store.ListServers(ctx)
	if err != nil {
		a.internal(ctx, err)
		return
	}
	a.health.mu.Lock()
	defer a.health.mu.Unlock()
	out := map[string]ServerHealth{}
	keep := map[string]*ServerHealth{}
	for _, v := range list {
		key := v.ID + "|" + v.Host + "|" + strconv.Itoa(v.SSHPort) // смена адреса - проверка с нуля
		h := a.health.m[key]
		if h == nil {
			h = &ServerHealth{State: "unknown"}
		}
		keep[key] = h
		if !h.checking && time.Since(h.CheckedAt) >= healthEvery {
			h.checking = true
			go a.checkHealth(h, v.Host, v.SSHPort)
		}
		out[v.ID] = *h
	}
	a.health.m = keep
	writeJSON(ctx, 200, out)
}

func (a *API) checkHealth(h *ServerHealth, host string, port int) {
	rtt, err := probeSSH(host, port)
	a.health.mu.Lock()
	defer a.health.mu.Unlock()
	h.checking, h.CheckedAt = false, time.Now()
	if err == nil {
		h.State, h.fails, h.Error, h.LatencyMs = "up", 0, "", int(rtt.Milliseconds())
		return
	}
	h.fails++
	h.Error = err.Error()
	if h.fails >= healthDownAt {
		h.State = "down"
	}
}
