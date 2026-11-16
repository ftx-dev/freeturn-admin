// Package sshx - SSH до VPS: вход паролем или ключом, проверка ключа хоста (TOFU), запуск команды
// со stdin. Без root команды идут через sudo; пароль sudo - первой строкой stdin.
package sshx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type Target struct {
	Host          string
	Port          int
	User          string
	Password      string // вход паролем
	PrivateKey    string // или ключом (PEM/OpenSSH)
	KeyPassphrase string
	SudoPassword  string
	// Fingerprint - доверенный SHA256-отпечаток ключа хоста. Пусто - доверяем первому (TOFU).
	Fingerprint string
}

// HostKeyChangedError - ключ хоста не совпал с запомненным: возможна подмена сервера.
type HostKeyChangedError struct{ Want, Got string }

func (e *HostKeyChangedError) Error() string {
	return fmt.Sprintf("ключ хоста изменился: был %s, сейчас %s", e.Want, e.Got)
}

type Client struct {
	c           *ssh.Client
	Fingerprint string // отпечаток, предъявленный сервером
	root        bool
	sudoPass    string
}

func Dial(ctx context.Context, t Target) (*Client, error) {
	var auth []ssh.AuthMethod
	switch {
	case t.PrivateKey != "":
		var signer ssh.Signer
		var err error
		if t.KeyPassphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(t.PrivateKey), []byte(t.KeyPassphrase))
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(t.PrivateKey))
		}
		if err != nil {
			return nil, fmt.Errorf("ключ SSH: %w", err)
		}
		auth = append(auth, ssh.PublicKeys(signer))
	case t.Password != "":
		pw := t.Password
		auth = append(auth, ssh.Password(pw), ssh.KeyboardInteractive(func(_, _ string, qs []string, _ []bool) ([]string, error) {
			out := make([]string, len(qs))
			for i := range out {
				out[i] = pw
			}
			return out, nil
		}))
	default:
		return nil, errors.New("не задан пароль или ключ SSH")
	}
	var presented string
	cfg := &ssh.ClientConfig{
		User: t.User,
		Auth: auth,
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			presented = ssh.FingerprintSHA256(key)
			if t.Fingerprint != "" && presented != t.Fingerprint {
				return &HostKeyChangedError{Want: t.Fingerprint, Got: presented}
			}
			return nil
		},
		Timeout: 15 * time.Second,
	}
	addr := net.JoinHostPort(strings.Trim(t.Host, "[]"), strconv.Itoa(t.Port))
	d := net.Dialer{Timeout: 15 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("нет соединения с %s: %w", addr, err)
	}
	if dl, ok := ctx.Deadline(); ok {
		conn.SetDeadline(dl)
	}
	cc, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	if err != nil {
		conn.Close()
		var hk *HostKeyChangedError
		if errors.As(err, &hk) {
			return nil, hk
		}
		if strings.Contains(err.Error(), "unable to authenticate") {
			return nil, errors.New("сервер не принял пароль или ключ SSH")
		}
		return nil, fmt.Errorf("SSH: %w", err)
	}
	conn.SetDeadline(time.Time{})
	c := &Client{c: ssh.NewClient(cc, chans, reqs), Fingerprint: presented, root: t.User == "root", sudoPass: t.SudoPassword}
	return c, nil
}

func (c *Client) Close() error { return c.c.Close() }

type Result struct {
	Stdout, Stderr []byte
	Code           int
}

// Run выполняет команду (строку для удалённой оболочки) с данными на stdin.
// asRoot - через sudo, если вход не под root.
func (c *Client) Run(ctx context.Context, command string, stdin []byte, asRoot bool) (Result, error) {
	s, err := c.c.NewSession()
	if err != nil {
		return Result{}, err
	}
	defer s.Close()
	if asRoot && !c.root {
		if c.sudoPass != "" {
			command = "sudo -S -p '' " + command
			stdin = append([]byte(c.sudoPass+"\n"), stdin...)
		} else {
			command = "sudo -n " + command
		}
	}
	var out, errb bytes.Buffer
	s.Stdout, s.Stderr = &out, &errb
	s.Stdin = bytes.NewReader(stdin)
	done := make(chan error, 1)
	go func() { done <- s.Run(command) }()
	select {
	case <-ctx.Done():
		s.Signal(ssh.SIGKILL)
		c.c.Close()
		return Result{}, fmt.Errorf("прервано: %w", ctx.Err())
	case err = <-done:
	}
	res := Result{Stdout: out.Bytes(), Stderr: errb.Bytes()}
	var exit *ssh.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exit):
		res.Code = exit.ExitStatus()
	default:
		return res, err
	}
	return res, nil
}

// Quote - аргумент для POSIX-оболочки в одинарных кавычках.
func Quote(s string) string {
	if s != "" && strings.IndexFunc(s, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("-_./=:@,+", r))
	}) < 0 {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
