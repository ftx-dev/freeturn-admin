package httpapi

import (
	"net"
	"testing"
	"time"

	"github.com/huhhuh78787878-collab/freeturn-admin/internal/sshx/sshtest"
)

func TestHealth(t *testing.T) {
	srv := sshtest.Start(t, "root", "pw")
	if _, err := probeSSH(srv.Host, srv.Port); err != nil {
		t.Fatal("живой SSH:", err)
	}
	ln, _ := net.Listen("tcp", "127.0.0.1:0") // порт открыт, но молчит
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			time.AfterFunc(6*time.Second, func() { c.Close() })
		}
	}()
	start := time.Now()
	if _, err := probeSSH("127.0.0.1", ln.Addr().(*net.TCPAddr).Port); err == nil || time.Since(start) > 6*time.Second {
		t.Fatal("молчащий порт должен давать ошибку по таймауту:", err)
	}

	// Одна неудача не гасит «up»; две подряд - «down»; успех сразу возвращает «up».
	a := &API{}
	h := &ServerHealth{State: "up"}
	a.checkHealth(h, "127.0.0.1", 1)
	if h.State != "up" {
		t.Fatal("после одной неудачи:", h.State)
	}
	a.checkHealth(h, "127.0.0.1", 1)
	if h.State != "down" {
		t.Fatal("после двух неудач:", h.State)
	}
	a.checkHealth(h, srv.Host, srv.Port)
	if h.State != "up" || h.fails != 0 {
		t.Fatal("после успеха:", h.State)
	}
}
