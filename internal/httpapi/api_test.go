package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"

	"github.com/huhhuh78787878-collab/freeturn-admin/internal/rpc"
	"github.com/huhhuh78787878-collab/freeturn-admin/internal/secret"
	"github.com/huhhuh78787878-collab/freeturn-admin/internal/store"
)

type client struct {
	t    *testing.T
	http *http.Client
	csrf string
}

func newAPI(t *testing.T) (*client, *store.Store) {
	t.Helper()
	st, err := store.Open(context.Background(), "sqlite", "file:"+filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	box, _ := secret.New(make([]byte, 32))
	script := filepath.Join(t.TempDir(), "install.sh")
	os.WriteFile(script, []byte(fakeInstall), 0o755)
	api := &API{Store: st, Box: box, Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Script: &rpc.Script{File: script},
		Web: fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}, "assets/a.js": {Data: []byte("js")}}}
	ln := fasthttputil.NewInmemoryListener()
	go fasthttp.Serve(ln, api.Handler())
	t.Cleanup(func() { ln.Close(); st.Close() })
	jar, _ := cookiejar.New(nil)
	hc := &http.Client{Jar: jar, Transport: &http.Transport{DialContext: func(context.Context, string, string) (net.Conn, error) { return ln.Dial() }}}
	return &client{t: t, http: hc}, st
}

// newClientFor - ещё один браузер (свои cookie) к тому же серверу.
func newClientFor(t *testing.T, c *client) (*client, error) {
	jar, _ := cookiejar.New(nil)
	return &client{t: t, http: &http.Client{Jar: jar, Transport: c.http.Transport}}, nil
}

func (c *client) do(method, path string, body any, out any) int {
	c.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = strings.NewReader(string(b))
	}
	req, _ := http.NewRequest(method, "http://admin"+path, r)
	if c.csrf != "" {
		req.Header.Set("X-CSRF-Token", c.csrf)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if out != nil {
		json.Unmarshal(data, out)
	}
	return resp.StatusCode
}

func TestAuthFlow(t *testing.T) {
	c, _ := newAPI(t)
	var state map[string]bool
	c.do("GET", "/api/auth/state", nil, &state)
	if !state["needsSetup"] {
		t.Fatal("ожидалась первичная настройка")
	}
	if code := c.do("GET", "/api/servers", nil, nil); code != 401 {
		t.Fatal("без входа:", code)
	}
	if code := c.do("POST", "/api/auth/setup", credentials{"admin", "short"}, nil); code != 422 {
		t.Fatal("короткий пароль:", code)
	}
	var me meResponse
	if code := c.do("POST", "/api/auth/setup", credentials{"admin", "long-enough-pass"}, &me); code != 200 || me.CSRF == "" {
		t.Fatal("setup:", code, me)
	}
	if code := c.do("POST", "/api/auth/setup", credentials{"evil", "long-enough-pass"}, nil); code != 409 {
		t.Fatal("второй setup:", code)
	}

	// Изменяющий запрос без CSRF-токена отклоняется, с токеном - проходит.
	if code := c.do("POST", "/api/servers", map[string]any{"name": "a", "host": "1.2.3.4", "password": "p"}, nil); code != 403 {
		t.Fatal("без CSRF:", code)
	}
	c.csrf = me.CSRF
	var srv map[string]any
	if code := c.do("POST", "/api/servers", map[string]any{"name": "Нидерланды", "host": "1.2.3.4", "password": "secret-pass"}, &srv); code != 201 {
		t.Fatal("create:", code, srv)
	}
	if srv["sshPort"].(float64) != 22 || srv["sshUser"] != "root" || srv["hasSecret"] != true {
		t.Fatal("значения по умолчанию:", srv)
	}
	var list []map[string]any
	c.do("GET", "/api/servers", nil, &list)
	raw, _ := json.Marshal(list)
	if len(list) != 1 || strings.Contains(string(raw), "secret-pass") || strings.Contains(string(raw), "Enc") {
		t.Fatal("секрет утёк или список неверный:", string(raw))
	}

	var e map[string]apiError
	if code := c.do("POST", "/api/servers", map[string]any{"name": "x", "host": "bad host!", "password": "p"}, &e); code != 422 || e["error"].Field != "host" {
		t.Fatal("плохой адрес:", code, e)
	}
	if code := c.do("POST", "/api/servers", map[string]any{"name": "x", "host": "h.example", "authType": "key", "privateKey": "nope"}, &e); code != 422 || e["error"].Field != "privateKey" {
		t.Fatal("плохой ключ:", code, e)
	}
	if code := c.do("POST", "/api/servers", map[string]any{"name": "x", "host": "h.example", "extra": 1}, nil); code != 400 {
		t.Fatal("лишнее поле:", code)
	}

	id := srv["id"].(string)
	if code := c.do("PATCH", "/api/servers/"+id, map[string]any{"name": "NL-1"}, &srv); code != 200 || srv["name"] != "NL-1" || srv["hasSecret"] != true {
		t.Fatal("patch:", code, srv)
	}
	if code := c.do("DELETE", "/api/servers/"+id, nil, nil); code != 200 {
		t.Fatal("delete:", code)
	}
	if code := c.do("GET", "/api/servers/"+id, nil, nil); code != 404 {
		t.Fatal("после удаления:", code)
	}

	var audit []store.AuditEntry
	c.do("GET", "/api/audit", nil, &audit)
	if len(audit) < 4 || audit[0].Action != "server.delete" {
		t.Fatal("журнал:", audit)
	}

	c.do("POST", "/api/auth/logout", nil, nil)
	if code := c.do("GET", "/api/auth/me", nil, nil); code != 401 {
		t.Fatal("после выхода:", code)
	}
}

func TestLoginRateLimit(t *testing.T) {
	c, _ := newAPI(t)
	c.do("POST", "/api/auth/setup", credentials{"admin", "long-enough-pass"}, nil)
	c2, _ := c, 0
	codes := map[int]int{}
	for i := 0; i < 12; i++ {
		codes[c2.do("POST", "/api/auth/login", credentials{"admin", "wrong-password"}, nil)]++
	}
	if codes[401] == 0 || codes[429] == 0 {
		t.Fatal("лимит не сработал:", codes)
	}
	if code := c2.do("POST", "/api/auth/login", credentials{"admin", "long-enough-pass"}, nil); code != 429 {
		t.Fatal("верный пароль во время блокировки:", code)
	}
}

func TestStaticFallback(t *testing.T) {
	c, _ := newAPI(t)
	req, _ := http.NewRequest("GET", "http://admin/servers/abc", nil)
	resp, err := c.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	if string(b) != "<html>app</html>" || resp.Header.Get("X-Frame-Options") != "DENY" {
		t.Fatal("SPA:", string(b), resp.Header)
	}
	if code := c.do("GET", "/api/nope", nil, nil); code != 404 {
		t.Fatal("api 404:", code)
	}
	if code := c.do("GET", "/../../etc/passwd", nil, nil); code != 200 {
		t.Fatal("обход пути:", code)
	}
}

func TestRoles(t *testing.T) {
	root, st := newAPI(t)
	var me meResponse
	root.do("POST", "/api/auth/setup", credentials{"owner", "long-enough-pass"}, &me)
	root.csrf = me.CSRF
	if me.Role != "root" {
		t.Fatal("первый администратор - не владелец:", me.Role)
	}
	var op, adm store.Admin
	if code := root.do("POST", "/api/admins", adminInput{"oper", "operator-pass-1", "operator"}, &op); code != 201 {
		t.Fatal("создать оператора:", code)
	}
	root.do("POST", "/api/admins", adminInput{"second", "admin-pass-123", "admin"}, &adm)
	if code := root.do("POST", "/api/admins", adminInput{"x", "admin-pass-123", "root"}, nil); code != 422 {
		t.Fatal("второго владельца создать нельзя:", code)
	}
	if code := root.do("POST", "/api/admins", adminInput{"oper", "operator-pass-1", "operator"}, nil); code != 422 {
		t.Fatal("дубликат логина:", code)
	}

	login := func(l, p string) *client {
		c, _ := newClientFor(t, root)
		var m meResponse
		if code := c.do("POST", "/api/auth/login", credentials{l, p}, &m); code != 200 {
			t.Fatal("вход", l, code)
		}
		c.csrf = m.CSRF
		return c
	}
	o := login("oper", "operator-pass-1")
	var list []store.Admin
	if code := o.do("GET", "/api/admins", nil, &list); code != 200 || len(list) != 3 {
		t.Fatal("оператор видит список:", code, len(list))
	}
	if code := o.do("POST", "/api/admins", adminInput{"evil", "operator-pass-1", "admin"}, nil); code != 403 {
		t.Fatal("оператор создал учётку:", code)
	}
	if code := o.do("DELETE", "/api/admins/"+adm.ID, nil, nil); code != 403 {
		t.Fatal("оператор удалил учётку:", code)
	}
	if code := o.do("POST", "/api/servers", map[string]any{"name": "s", "host": "1.2.3.4", "password": "p"}, nil); code != 201 {
		t.Fatal("оператор не может добавить сервер:", code)
	}

	a := login("second", "admin-pass-123")
	rootID := ""
	for _, x := range list {
		if x.Role == "root" {
			rootID = x.ID
		}
	}
	if code := a.do("DELETE", "/api/admins/"+rootID, nil, nil); code != 403 {
		t.Fatal("администратор удалил владельца:", code)
	}
	if code := a.do("PATCH", "/api/admins/"+op.ID, map[string]string{"role": "admin"}, nil); code != 200 {
		t.Fatal("повысить оператора:", code)
	}
	if code := o.do("POST", "/api/admins", adminInput{"third", "operator-pass-1", "operator"}, nil); code != 201 {
		t.Fatal("роль применяется сразу, без перевхода:", code)
	}
	if code := a.do("DELETE", "/api/admins/"+op.ID, nil, nil); code != 200 {
		t.Fatal("удалить:", code)
	}
	if code := o.do("GET", "/api/auth/me", nil, nil); code != 401 {
		t.Fatal("сессия удалённой учётки жива:", code)
	}

	if code := a.do("POST", "/api/auth/password", map[string]string{"current": "wrong", "new": "new-password-123"}, nil); code != 422 {
		t.Fatal("смена пароля с неверным текущим:", code)
	}
	if code := a.do("POST", "/api/auth/password", map[string]string{"current": "admin-pass-123", "new": "new-password-123"}, nil); code != 200 {
		t.Fatal("смена пароля:", code)
	}
	if code := a.do("GET", "/api/auth/me", nil, nil); code != 200 {
		t.Fatal("текущая сессия после смены пароля должна жить:", code)
	}
	_ = st
}

func TestLinkWithCall(t *testing.T) {
	c, _ := newAPI(t)
	var me meResponse
	c.do("POST", "/api/auth/setup", credentials{"owner", "long-enough-pass"}, &me)
	c.csrf = me.CSRF
	base := "freeturn://eyJ2IjoxLCJwcm92aWRlciI6InZrIiwicGVlciI6IjEuMi4zLjQ6NTYwMDAiLCJuYW1lIjoiYm9iIn0"
	var out map[string]string
	if code := c.do("POST", "/api/links/call", map[string]string{"link": base, "callLink": "https://vk.com/call/join/x"}, &out); code != 200 || out["link"] == base {
		t.Fatal("вложить:", code, out)
	}
	var back map[string]string
	c.do("POST", "/api/links/call", map[string]string{"link": out["link"], "callLink": ""}, &back)
	if back["link"] != base {
		t.Fatal("убрать звонок - вернуть исходную ссылку:", back["link"])
	}
	var e map[string]apiError
	if code := c.do("POST", "/api/links/call", map[string]string{"link": base, "callLink": "http://x"}, &e); code != 422 || e["error"].Field != "callLink" {
		t.Fatal("плохая ссылка на звонок:", code, e)
	}
}
