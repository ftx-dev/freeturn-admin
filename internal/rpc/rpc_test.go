package rpc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/huhhuh78787878-collab/freeturn-admin/internal/sshx"
	"github.com/huhhuh78787878-collab/freeturn-admin/internal/sshx/sshtest"
)

// Поддельный install.sh: отвечает как proto 3 и возвращает аргументы, чтобы проверить экранирование.
const fakeScript = `#!/bin/bash
cmd=$1; shift
esc() { local s=$1; s=${s//\\/\\\\}; s=${s//\"/\\\"}; printf '%s' "$s"; }
args=""; for a in "$@"; do args="$args${args:+,}\"$(esc "$a")\""; done
echo "noise from apt-get"
case $cmd in
  probe) echo "{\"proto\":3,\"result\":\"ok\",\"data\":{\"installed\":true,\"running\":true,\"euid\":$(id -u),\"arch\":\"x86_64\",\"config\":{\"method\":\"docker\",\"listen_port\":56123}},\"logs\":[]}" ;;
  client-add) echo "{\"proto\":3,\"result\":\"ok\",\"data\":{\"client\":{\"name\":\"bob\",\"client_id\":\"c1\",\"self\":false},\"link\":\"freeturn://x\",\"conf_b64\":\"\",\"args\":[$args]},\"logs\":[]}" ;;
  client-remove) echo "{\"proto\":3,\"result\":\"err\",\"code\":\"not_found\",\"msg\":\"клиента нет\",\"stage\":\"client_remove\",\"logs\":[]}" ;;
  *) exit 3 ;;
esac
`

func dial(t *testing.T, srv *sshtest.Server, user, pass, sudo string) *sshx.Client {
	t.Helper()
	c, err := sshx.Dial(context.Background(), sshx.Target{Host: srv.Host, Port: srv.Port, User: user, Password: pass, SudoPassword: sudo})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func TestCallRoot(t *testing.T) {
	srv := sshtest.Start(t, "root", "pw")
	c := dial(t, srv, "root", "pw", "")
	if c.Fingerprint != srv.Fingerprint {
		t.Fatal("отпечаток", c.Fingerprint, srv.Fingerprint)
	}
	var p Probe
	if err := Call(context.Background(), c, []byte(fakeScript), "probe", nil, &p); err != nil {
		t.Fatal(err)
	}
	if !p.Installed || !p.Running || p.Arch != "x86_64" || p.Config == nil || p.Config.ListenPort != 56123 {
		t.Fatalf("%+v", p)
	}

	var add struct {
		ClientConf
		Args []string `json:"args"`
	}
	tricky := `it's "x" $(rm -rf /) ; \`
	if err := Call(context.Background(), c, []byte(fakeScript), "client-add", map[string]string{"name": tricky, "purge": ""}, &add); err != nil {
		t.Fatal(err)
	}
	if add.Link != "freeturn://x" || len(add.Args) != 2 || add.Args[0] != "--name="+tricky || add.Args[1] != "--purge" {
		t.Fatalf("аргументы дошли не как есть: %q", add.Args)
	}

	var e *Error
	if err := Call(context.Background(), c, []byte(fakeScript), "client-remove", map[string]string{"name": "x"}, nil); !errors.As(err, &e) || e.Code != "not_found" {
		t.Fatal(err)
	}
	if err := Call(context.Background(), c, []byte(fakeScript), "stop", nil, nil); !errors.As(err, &e) || e.Code != "no_response" {
		t.Fatal("нет JSON в ответе:", err)
	}
	if err := Call(context.Background(), c, nil, "rm", nil, nil); err == nil {
		t.Fatal("неизвестная команда принята")
	}
	if err := Call(context.Background(), c, nil, "probe", map[string]string{"x; rm": "1"}, nil); err == nil {
		t.Fatal("недопустимый ключ принят")
	}
}

func TestSudoAndHostKey(t *testing.T) {
	srv := sshtest.Start(t, "deploy", "pw")
	srv.Env = []string{sshtest.FakeSudo(t, t.TempDir(), "sudo-secret")}
	var p Probe
	// probe - без sudo; остальные команды - через sudo с паролем из stdin.
	if err := Call(context.Background(), dial(t, srv, "deploy", "pw", "sudo-secret"), []byte(fakeScript), "client-add", map[string]string{"name": "bob"}, &p); err != nil {
		t.Fatal("sudo с паролем:", err)
	}
	var e *Error
	err := Call(context.Background(), dial(t, srv, "deploy", "pw", "wrong"), []byte(fakeScript), "client-add", map[string]string{"name": "bob"}, nil)
	if !errors.As(err, &e) || e.Code != "sudo" {
		t.Fatal("неверный пароль sudo:", err)
	}

	_, err = sshx.Dial(context.Background(), sshx.Target{Host: srv.Host, Port: srv.Port, User: "deploy", Password: "pw", Fingerprint: "SHA256:other"})
	var hk *sshx.HostKeyChangedError
	if !errors.As(err, &hk) || hk.Got != srv.Fingerprint {
		t.Fatal("смена ключа хоста не замечена:", err)
	}
	_, err = sshx.Dial(context.Background(), sshx.Target{Host: srv.Host, Port: srv.Port, User: "deploy", Password: "bad"})
	if err == nil || !strings.Contains(err.Error(), "не принял") {
		t.Fatal("неверный пароль:", err)
	}
}

func TestQuote(t *testing.T) {
	for in, want := range map[string]string{"abc": "abc", "--name=bob": "--name=bob", "": "''", "a b": "'a b'", "it's": `'it'\''s'`} {
		if got := sshx.Quote(in); got != want {
			t.Errorf("Quote(%q) = %q, want %q", in, got, want)
		}
	}
}
