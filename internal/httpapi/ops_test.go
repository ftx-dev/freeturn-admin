package httpapi

import (
	"testing"
	"time"

	"github.com/huhhuh78787878-collab/freeturn-admin/internal/sshx/sshtest"
)

// Поддельный install.sh для тестов API.
const fakeInstall = `#!/bin/bash
cmd=$1; shift
ok() { echo "{\"proto\":3,\"result\":\"ok\",\"data\":{$1},\"logs\":[]}"; }
case $cmd in
  probe) ok '"installed":true,"running":true,"euid":0,"arch":"x86_64","config":{"listen_port":56001,"obf_profile":"rtpopus3"}' ;;
  client-list) ok '"clients":[{"name":"owner","client_id":"c0","self":true,"ip":"10.66.0.2","hs":0}],"share":{"backend":"new","port":56001}' ;;
  client-add) ok '"client":{"name":"bob","client_id":"c1","self":false},"link":"freeturn://bob","conf_b64":""' ;;
  apply) sleep 0.2; ok '"owner":{"client_id":"c0","link":"freeturn://owner"},"obf_key":"k","needs_restart":false,"args":"'"$*"'"' ;;
  *) ok '' ;;
esac
`

func TestServerOps(t *testing.T) {
	c, _ := newAPI(t)
	var me meResponse
	c.do("POST", "/api/auth/setup", credentials{"admin", "long-enough-pass"}, &me)
	c.csrf = me.CSRF
	srv := sshtest.Start(t, "root", "pw")

	var s map[string]any
	c.do("POST", "/api/servers", map[string]any{"name": "test", "host": srv.Host, "sshPort": srv.Port, "password": "pw"}, &s)
	id := s["id"].(string)

	var test map[string]any
	if code := c.do("POST", "/api/servers/"+id+"/ssh/test", nil, &test); code != 200 || test["fingerprint"] != srv.Fingerprint {
		t.Fatal("ssh test:", code, test)
	}
	c.do("GET", "/api/servers/"+id, nil, &s)
	if s["hostFingerprint"] != srv.Fingerprint {
		t.Fatal("ключ хоста не запомнен (TOFU):", s)
	}

	var users map[string]any
	if code := c.do("GET", "/api/servers/"+id+"/users", nil, &users); code != 200 || len(users["clients"].([]any)) != 1 {
		t.Fatal("users:", code, users)
	}
	var e map[string]apiError
	if code := c.do("POST", "/api/servers/"+id+"/users", map[string]string{"name": "bad name"}, &e); code != 422 {
		t.Fatal("плохое имя:", code)
	}
	var added map[string]any
	if code := c.do("POST", "/api/servers/"+id+"/users", map[string]string{"name": "bob"}, &added); code != 200 || added["link"] != "freeturn://bob" {
		t.Fatal("add:", code, added)
	}

	if code := c.do("POST", "/api/servers/"+id+"/apply", map[string]any{"backend": "new", "mode": "tcp"}, &e); code != 422 || e["error"].Field != "mode" {
		t.Fatal("tcp без своего VPN:", code, e)
	}
	var job map[string]any
	if code := c.do("POST", "/api/servers/"+id+"/apply", map[string]any{"method": "docker", "backend": "new", "listenPort": 56001, "obfProfile": "rtpopus3"}, &job); code != 202 {
		t.Fatal("apply:", code, job)
	}
	if code := c.do("POST", "/api/servers/"+id+"/apply", map[string]any{}, &e); code != 409 {
		t.Fatal("вторая задача параллельно:", code)
	}
	for i := 0; job["status"] == "running" && i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		c.do("GET", "/api/jobs/"+job["id"].(string), nil, &job)
	}
	res, _ := job["result"].(map[string]any)
	if job["status"] != "ok" || res["owner"].(map[string]any)["link"] != "freeturn://owner" {
		t.Fatal("задача:", job)
	}

	// Другой сервер на том же адресе - другой ключ хоста: операции блокируются до подтверждения.
	other := sshtest.Start(t, "root", "pw")
	c.do("PATCH", "/api/servers/"+id, map[string]any{"sshPort": other.Port}, &s)
	c.do("POST", "/api/servers/"+id+"/ssh/test", nil, nil) // новый адрес - отпечаток сброшен и запомнен заново
	c.do("PATCH", "/api/servers/"+id, map[string]any{"sshPort": srv.Port}, nil)
	c.do("POST", "/api/servers/"+id+"/ssh/trust", map[string]string{"fingerprint": other.Fingerprint}, nil)
	var changed map[string]any
	if code := c.do("GET", "/api/servers/"+id+"/status", nil, &changed); code != 409 || changed["got"] != srv.Fingerprint {
		t.Fatal("смена ключа:", code, changed)
	}
	if code := c.do("POST", "/api/servers/"+id+"/ssh/trust", map[string]string{"fingerprint": srv.Fingerprint}, nil); code != 200 {
		t.Fatal("trust:", code)
	}
	if code := c.do("GET", "/api/servers/"+id+"/status", nil, nil); code != 200 {
		t.Fatal("после подтверждения:", code)
	}
}

func TestApplySettingsValidation(t *testing.T) {
	cases := []struct {
		in    ApplySettings
		field string
	}{
		{ApplySettings{Method: "podman"}, "method"},
		{ApplySettings{Backend: "external"}, "connect"},
		{ApplySettings{Backend: "external", Connect: "vpn.example:51820", Mode: "tcp"}, ""},
		{ApplySettings{ListenPort: 80}, "listenPort"},
		{ApplySettings{ListenPort: 56001, WGPort: 56001}, "wgPort"},
		{ApplySettings{WGNet: "10.0.0.0/8"}, "wgNet"},
		{ApplySettings{WGNet: "10.66.0.0/24"}, ""},
		{ApplySettings{ObfKey: "xyz"}, "obfKey"},
		{ApplySettings{Host: "bad host"}, "host"},
	}
	for _, c := range cases {
		if _, field, _ := c.in.opts(); field != c.field {
			t.Errorf("%+v: поле %q, ожидалось %q", c.in, field, c.field)
		}
	}
	if p := RandomListenPort(); p < 56000 || p > 56999 || len(NewObfKey()) != 64 {
		t.Fatal("значения по умолчанию")
	}
}
