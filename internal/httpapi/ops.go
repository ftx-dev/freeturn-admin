package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"math/big"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/huhhuh78787878-collab/freeturn-admin/internal/auth"
	"github.com/huhhuh78787878-collab/freeturn-admin/internal/rpc"
	"github.com/huhhuh78787878-collab/freeturn-admin/internal/sshx"
	"github.com/huhhuh78787878-collab/freeturn-admin/internal/store"
)

var (
	clientNameRe  = regexp.MustCompile(`^[A-Za-z0-9._-]{1,32}$`)
	fingerprintRe = regexp.MustCompile(`^SHA256:[A-Za-z0-9+/]{43}$`)
	hex64Re       = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)
)

// target собирает параметры SSH с расшифрованными секретами.
func (a *API) target(v store.Server) (sshx.Target, error) {
	t := sshx.Target{Host: v.Host, Port: v.SSHPort, User: v.SSHUser, Fingerprint: v.HostFingerprint}
	secret, err := a.Box.Open(v.SSHSecretEnc)
	if err != nil {
		return t, err
	}
	if v.AuthType == "key" {
		t.PrivateKey = secret
	} else {
		t.Password = secret
	}
	if t.KeyPassphrase, err = a.Box.Open(v.KeyPassphraseEnc); err != nil {
		return t, err
	}
	t.SudoPassword, err = a.Box.Open(v.SudoSecretEnc)
	return t, err
}

// dial подключается к серверу. Первый увиденный ключ хоста запоминается (TOFU);
// другой ключ - ошибка до явного подтверждения через /ssh/trust.
func (a *API) dial(ctx context.Context, v *store.Server) (*sshx.Client, error) {
	t, err := a.target(*v)
	if err != nil {
		return nil, err
	}
	c, err := sshx.Dial(ctx, t)
	if err != nil {
		return nil, err
	}
	if v.HostFingerprint == "" {
		v.HostFingerprint = c.Fingerprint
		if err := a.Store.UpdateServer(ctx, *v); err != nil {
			c.Close()
			return nil, err
		}
	}
	return c, nil
}

// sshFail переводит ошибки SSH и install.sh в ответ API.
func (a *API) sshFail(ctx *fasthttp.RequestCtx, err error) {
	var hk *sshx.HostKeyChangedError
	var re *rpc.Error
	switch {
	case errors.As(err, &hk):
		writeJSON(ctx, 409, map[string]any{"error": apiError{Code: "host_key_changed",
			Message: "Ключ хоста сервера изменился. Если сервер переустанавливали - подтвердите новый ключ, иначе это может быть подмена."},
			"want": hk.Want, "got": hk.Got})
	case errors.As(err, &re):
		writeJSON(ctx, 502, map[string]any{"error": apiError{Code: "server_" + re.Code, Message: re.Msg}, "stage": re.Stage, "logs": re.Logs})
	case errors.Is(err, store.ErrNotFound):
		fail(ctx, 404, "not_found", "Не найдено")
	default:
		fail(ctx, 502, "ssh", err.Error())
	}
}

// withServer: загрузить сервер, подключиться по SSH, выполнить fn. Таймаут - на всю операцию.
func (a *API) withServer(timeout time.Duration, fn func(ctx context.Context, v *store.Server, c *sshx.Client, script []byte) (any, error)) fasthttp.RequestHandler {
	return func(rc *fasthttp.RequestCtx) {
		v, err := a.Store.GetServer(rc, rc.UserValue("id").(string))
		if err != nil {
			a.internal(rc, err)
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		script, err := a.Script.Get(ctx)
		if err != nil {
			fail(rc, 502, "script", err.Error())
			return
		}
		c, err := a.dial(ctx, &v)
		if err != nil {
			a.sshFail(rc, err)
			return
		}
		defer c.Close()
		out, err := fn(ctx, &v, c, script)
		if err != nil {
			a.sshFail(rc, err)
			return
		}
		writeJSON(rc, 200, out)
	}
}

func call(cmd string, opts map[string]string, out any) func(context.Context, *store.Server, *sshx.Client, []byte) (any, error) {
	return func(ctx context.Context, _ *store.Server, c *sshx.Client, script []byte) (any, error) {
		return out, rpc.Call(ctx, c, script, cmd, opts, out)
	}
}

func (a *API) sshTest(rc *fasthttp.RequestCtx) {
	a.withServer(40*time.Second, func(ctx context.Context, v *store.Server, c *sshx.Client, script []byte) (any, error) {
		var p rpc.Probe
		err := rpc.Call(ctx, c, script, "probe", nil, &p)
		a.audit(rc, v.ID, "server.ssh_test", map[string]string{"fingerprint": c.Fingerprint}, resultOf(err))
		return map[string]any{"fingerprint": c.Fingerprint, "probe": p}, err
	})(rc)
}

// sshTrust - администратор подтвердил новый ключ хоста (сервер переустановлен).
func (a *API) sshTrust(rc *fasthttp.RequestCtx) {
	var in struct {
		Fingerprint string `json:"fingerprint"`
	}
	if !readJSON(rc, &in) {
		return
	}
	if !fingerprintRe.MatchString(in.Fingerprint) {
		failField(rc, "fingerprint", "Отпечаток в формате SHA256:…")
		return
	}
	v, err := a.Store.GetServer(rc, rc.UserValue("id").(string))
	if err != nil {
		a.internal(rc, err)
		return
	}
	old := v.HostFingerprint
	v.HostFingerprint = in.Fingerprint
	if err := a.Store.UpdateServer(rc, v); err != nil {
		a.internal(rc, err)
		return
	}
	a.audit(rc, v.ID, "server.trust", map[string]string{"old": old, "new": in.Fingerprint}, "ok")
	writeJSON(rc, 200, view(v))
}

func (a *API) status(rc *fasthttp.RequestCtx) {
	a.withServer(40*time.Second, call("probe", nil, &rpc.Probe{}))(rc)
}

func (a *API) serverLogs(rc *fasthttp.RequestCtx) {
	tail := rc.QueryArgs().GetUintOrZero("tail")
	if tail <= 0 || tail > 2000 {
		tail = 200
	}
	a.withServer(40*time.Second, call("logs", map[string]string{"tail": strconv.Itoa(tail)}, &rpc.Logs{}))(rc)
}

func (a *API) listUsers(rc *fasthttp.RequestCtx) {
	a.withServer(40*time.Second, call("client-list", nil, &rpc.ClientList{}))(rc)
}

func (a *API) getUser(rc *fasthttp.RequestCtx) {
	name := rc.UserValue("name").(string)
	if !clientNameRe.MatchString(name) {
		fail(rc, 404, "not_found", "Нет такого пользователя")
		return
	}
	a.withServer(40*time.Second, call("client-conf", map[string]string{"name": name}, &rpc.ClientConf{}))(rc)
}

func (a *API) addUser(rc *fasthttp.RequestCtx) {
	var in struct {
		Name string `json:"name"`
	}
	if !readJSON(rc, &in) {
		return
	}
	if !clientNameRe.MatchString(in.Name) {
		failField(rc, "name", "Имя: латиница, цифры, . _ - (до 32)")
		return
	}
	a.withServer(60*time.Second, func(ctx context.Context, v *store.Server, c *sshx.Client, script []byte) (any, error) {
		var out rpc.ClientConf
		err := rpc.Call(ctx, c, script, "client-add", map[string]string{"name": in.Name}, &out)
		a.audit(rc, v.ID, "user.add", map[string]string{"name": in.Name}, resultOf(err))
		return out, err
	})(rc)
}

func (a *API) removeUser(rc *fasthttp.RequestCtx) {
	name := rc.UserValue("name").(string)
	if !clientNameRe.MatchString(name) {
		fail(rc, 404, "not_found", "Нет такого пользователя")
		return
	}
	a.withServer(60*time.Second, func(ctx context.Context, v *store.Server, c *sshx.Client, script []byte) (any, error) {
		err := rpc.Call(ctx, c, script, "client-remove", map[string]string{"name": name}, nil)
		a.audit(rc, v.ID, "user.remove", map[string]string{"name": name}, resultOf(err))
		return map[string]bool{"ok": true}, err
	})(rc)
}

func (a *API) simple(cmd, action string) fasthttp.RequestHandler {
	return func(rc *fasthttp.RequestCtx) {
		a.withServer(2*time.Minute, func(ctx context.Context, v *store.Server, c *sshx.Client, script []byte) (any, error) {
			err := rpc.Call(ctx, c, script, cmd, nil, nil)
			a.audit(rc, v.ID, action, nil, resultOf(err))
			return map[string]bool{"ok": true}, err
		})(rc)
	}
}

// ---- Установка и применение настроек - фоновой задачей ----

// ApplySettings - параметры install.sh apply. Пустые поля = значения сервера / install.sh по умолчанию.
type ApplySettings struct {
	Method     string `json:"method"`     // docker | systemd
	Backend    string `json:"backend"`    // new | external
	Connect    string `json:"connect"`    // host:port для external
	Mode       string `json:"mode"`       // udp | tcp
	ListenPort int    `json:"listenPort"` // 0 - случайный 56000-56999 (при первой установке)
	WGPort     int    `json:"wgPort"`
	WGNet      string `json:"wgNet"`
	ObfProfile string `json:"obfProfile"` // rtpopus3 | rtpopus2 | rtpopus | none
	ObfKey     string `json:"obfKey"`     // 64 hex; пусто - сгенерируется
	Host       string `json:"host"`       // адрес в ссылках; пусто - внешний IP
	Update     bool   `json:"update"`     // обновить FreeTurn до последнего релиза
}

// opts проверяет настройки и переводит их в ключи install.sh. Ошибка - (поле, сообщение).
func (s ApplySettings) opts() (map[string]string, string, string) {
	o := map[string]string{}
	inSet := func(v string, allowed ...string) bool {
		for _, x := range allowed {
			if v == x {
				return true
			}
		}
		return false
	}
	port := func(p int) bool { return p >= 1024 && p <= 65535 }
	if s.Method != "" {
		if !inSet(s.Method, "docker", "systemd") {
			return nil, "method", "Запуск: docker или systemd"
		}
		o["method"] = s.Method
	}
	if s.Backend != "" {
		if !inSet(s.Backend, "new", "external") {
			return nil, "backend", "Куда идёт трафик: new или external"
		}
		o["backend"] = s.Backend
	}
	if s.Backend == "external" {
		if _, p, err := net.SplitHostPort(s.Connect); err != nil || p == "" {
			return nil, "connect", "Адрес своего VPN: host:port"
		}
		o["connect"] = s.Connect
	}
	if s.Mode != "" {
		if !inSet(s.Mode, "udp", "tcp") {
			return nil, "mode", "Режим: udp или tcp"
		}
		if s.Mode == "tcp" && s.Backend != "external" {
			return nil, "mode", "TCP (Xray/VLESS) - только для своего VPN"
		}
		o["mode"] = s.Mode
	}
	if s.ListenPort != 0 {
		if !port(s.ListenPort) {
			return nil, "listenPort", "Порт FreeTurn: 1024–65535"
		}
		o["listen-port"] = strconv.Itoa(s.ListenPort)
	}
	if s.WGPort != 0 {
		if !port(s.WGPort) || s.WGPort == s.ListenPort {
			return nil, "wgPort", "Порт WireGuard: 1024–65535, не равный порту FreeTurn"
		}
		o["wg-port"] = strconv.Itoa(s.WGPort)
	}
	if s.WGNet != "" {
		ip, n, err := net.ParseCIDR(s.WGNet)
		ones := 0
		if n != nil {
			ones, _ = n.Mask.Size()
		}
		if err != nil || ip.To4() == nil || ones < 16 || ones > 29 {
			return nil, "wgNet", "Подсеть WireGuard: IPv4 от /16 до /29"
		}
		o["wg-net"] = s.WGNet
	}
	if s.ObfProfile != "" {
		if !inSet(s.ObfProfile, "rtpopus3", "rtpopus2", "rtpopus", "none") {
			return nil, "obfProfile", "Обфускация: rtpopus3, rtpopus2, rtpopus или none"
		}
		o["obf-profile"] = s.ObfProfile
	}
	if s.ObfKey != "" {
		if !hex64Re.MatchString(s.ObfKey) {
			return nil, "obfKey", "Ключ обфускации: 64 hex-символа"
		}
		o["obf-key"] = strings.ToLower(s.ObfKey)
	}
	if s.Host != "" {
		if !validHost(s.Host) {
			return nil, "host", "Адрес в ссылках: домен или IP"
		}
		o["host"] = s.Host
	}
	if s.Update {
		o["update"] = ""
	}
	return o, "", ""
}

// RandomListenPort - порт FreeTurn по умолчанию, как у Android-клиента: случайный из 56000-56999.
func RandomListenPort() int {
	n, _ := rand.Int(rand.Reader, big.NewInt(1000))
	return 56000 + int(n.Int64())
}

func NewObfKey() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (a *API) applyServer(rc *fasthttp.RequestCtx) {
	var s ApplySettings
	if !readJSON(rc, &s) {
		return
	}
	opts, field, msg := s.opts()
	if field != "" {
		failField(rc, field, msg)
		return
	}
	a.startJob(rc, "apply", func(ctx context.Context, c *sshx.Client, script []byte) (any, error) {
		var out rpc.Apply
		return &out, rpc.Call(ctx, c, script, "apply", opts, &out)
	})
}

func (a *API) uninstall(rc *fasthttp.RequestCtx) {
	var in struct {
		Target string `json:"target"`
		Purge  bool   `json:"purge"`
	}
	if !readJSON(rc, &in) {
		return
	}
	if in.Target != "freeturn" && in.Target != "wg" && in.Target != "all" {
		failField(rc, "target", "Что удалить: freeturn, wg или all")
		return
	}
	opts := map[string]string{"target": in.Target}
	if in.Purge {
		opts["purge"] = ""
	}
	a.startJob(rc, "uninstall", func(ctx context.Context, c *sshx.Client, script []byte) (any, error) {
		return map[string]bool{"ok": true}, rpc.Call(ctx, c, script, "uninstall", opts, nil)
	})
}

// ---- Задачи ----

// shortcut: задачи живут в памяти процесса (после перезапуска история пропадает); перенести в таблицу jobs из плана.
type Job struct {
	ID         string    `json:"id"`
	ServerID   string    `json:"serverId"`
	Kind       string    `json:"kind"`
	Status     string    `json:"status"` // running | ok | error
	Error      string    `json:"error,omitempty"`
	ErrorCode  string    `json:"errorCode,omitempty"`
	Stage      string    `json:"stage,omitempty"`
	Logs       []string  `json:"logs,omitempty"`
	Result     any       `json:"result,omitempty"`
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt,omitzero"`
}

type jobs struct {
	mu   sync.Mutex
	byID map[string]*Job
	busy map[string]string // serverID -> id идущей задачи: на сервере одна задача за раз
}

func (a *API) startJob(rc *fasthttp.RequestCtx, kind string, fn func(ctx context.Context, c *sshx.Client, script []byte) (any, error)) {
	v, err := a.Store.GetServer(rc, rc.UserValue("id").(string))
	if err != nil {
		a.internal(rc, err)
		return
	}
	a.jobs.mu.Lock()
	if id, ok := a.jobs.busy[v.ID]; ok {
		a.jobs.mu.Unlock()
		writeJSON(rc, 409, map[string]any{"error": apiError{Code: "busy", Message: "На сервере уже идёт операция"}, "jobId": id})
		return
	}
	j := &Job{ID: auth.ID(), ServerID: v.ID, Kind: kind, Status: "running", StartedAt: time.Now()}
	a.jobs.byID[j.ID] = j
	a.jobs.busy[v.ID] = j.ID
	a.jobs.mu.Unlock()
	admin := adminOf(rc)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		result, err := func() (any, error) {
			script, err := a.Script.Get(ctx)
			if err != nil {
				return nil, err
			}
			c, err := a.dial(ctx, &v)
			if err != nil {
				return nil, err
			}
			defer c.Close()
			return fn(ctx, c, script)
		}()
		a.jobs.mu.Lock()
		j.FinishedAt = time.Now()
		if err != nil {
			j.Status, j.Error = "error", err.Error()
			var re *rpc.Error
			if errors.As(err, &re) {
				j.ErrorCode, j.Stage, j.Logs = re.Code, re.Stage, re.Logs
			}
		} else {
			j.Status, j.Result = "ok", result
		}
		delete(a.jobs.busy, v.ID)
		a.jobs.mu.Unlock()
		if err := a.Store.Audit(context.Background(), admin.ID, v.ID, "server."+kind, `{}`, resultOf(err)); err != nil {
			a.Log.Error("audit", "err", err)
		}
	}()
	a.jobs.mu.Lock()
	defer a.jobs.mu.Unlock()
	writeJSON(rc, 202, j)
}

func (a *API) getJob(rc *fasthttp.RequestCtx) {
	a.jobs.mu.Lock()
	defer a.jobs.mu.Unlock()
	j, ok := a.jobs.byID[rc.UserValue("id").(string)]
	if !ok {
		fail(rc, 404, "not_found", "Задача не найдена")
		return
	}
	writeJSON(rc, 200, j)
}

func resultOf(err error) string {
	if err != nil {
		return "fail"
	}
	return "ok"
}
