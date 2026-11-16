// Package sshtest - SSH-сервер для тестов: принимает пароль, выполняет команду локальным bash
// со stdin клиента и возвращает код выхода.
package sshtest

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"testing"

	"golang.org/x/crypto/ssh"
)

type Server struct {
	Host        string
	Port        int
	Fingerprint string
	Env         []string // добавляется к окружению команд (например, PATH с поддельным sudo)
}

func Start(t *testing.T, user, password string) *Server {
	t.Helper()
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := ssh.NewSignerFromKey(priv)
	cfg := &ssh.ServerConfig{PasswordCallback: func(c ssh.ConnMetadata, p []byte) (*ssh.Permissions, error) {
		if c.User() == user && string(p) == password {
			return nil, nil
		}
		return nil, io.EOF
	}}
	cfg.AddHostKey(signer)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	s := &Server{Host: "127.0.0.1", Port: ln.Addr().(*net.TCPAddr).Port, Fingerprint: ssh.FingerprintSHA256(signer.PublicKey())}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go s.serve(conn, cfg)
		}
	}()
	return s
}

func (s *Server) serve(conn net.Conn, cfg *ssh.ServerConfig) {
	_, chans, reqs, err := ssh.NewServerConn(conn, cfg)
	if err != nil {
		return
	}
	go ssh.DiscardRequests(reqs)
	for nc := range chans {
		ch, creqs, err := nc.Accept()
		if err != nil {
			continue
		}
		go func() {
			defer ch.Close()
			for req := range creqs {
				if req.Type != "exec" {
					req.Reply(false, nil)
					continue
				}
				n := binary.BigEndian.Uint32(req.Payload)
				command := string(req.Payload[4 : 4+n])
				req.Reply(true, nil)
				cmd := exec.Command("bash", "-c", command)
				cmd.Env = append(os.Environ(), s.Env...)
				cmd.Stdin, cmd.Stdout, cmd.Stderr = ch, ch, ch.Stderr()
				code := 0
				if err := cmd.Run(); err != nil {
					code = 255
					if ee, ok := err.(*exec.ExitError); ok {
						code = ee.ExitCode()
					}
				}
				ch.SendRequest("exit-status", false, binary.BigEndian.AppendUint32(nil, uint32(code)))
				return
			}
		}()
	}
}

// FakeSudo кладёт в dir скрипт sudo, который проверяет пароль из первой строки stdin.
func FakeSudo(t *testing.T, dir, password string) string {
	t.Helper()
	script := `#!/bin/bash
while [ "${1#-}" != "$1" ]; do [ "$1" = -p ] && shift; shift; done
read -r pw
[ "$pw" = ` + strconv.Quote(password) + ` ] || { echo "sudo: incorrect password" >&2; exit 1; }
exec "$@"
`
	if err := os.WriteFile(dir+"/sudo", []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return "PATH=" + dir + ":" + os.Getenv("PATH")
}
