// Package rpc вызывает install.sh ядра на VPS как JSON RPC (proto 3): probe, apply, client-* и т.д.
// Скрипт передаётся через stdin (bash -s), на сервере ничего не скачивается заранее.
package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/huhhuh78787878-collab/freeturn-admin/internal/sshx"
)

const Proto = 3

// Error - ответ result=err от install.sh.
type Error struct {
	Code, Msg, Stage string
	Logs             []string
}

func (e *Error) Error() string { return e.Msg }

type response struct {
	Proto  int             `json:"proto"`
	Result string          `json:"result"`
	Data   json.RawMessage `json:"data"`
	Code   string          `json:"code"`
	Msg    string          `json:"msg"`
	Stage  string          `json:"stage"`
	Logs   []string        `json:"logs"`
}

// Script отдаёт текст install.sh: из файла (FTA_INSTALL_SCRIPT) или с GitHub, с кэшем на час.
type Script struct {
	URL  string // raw-ссылка на install.sh
	File string // локальный файл вместо URL

	mu      sync.Mutex
	body    []byte
	fetched time.Time
}

func ScriptURL(ref string) string {
	if ref == "" || ref == "latest" {
		ref = "master"
	}
	return "https://raw.githubusercontent.com/hackdiaz-dev/free-turn-proxy/" + ref + "/scripts/install.sh"
}

func (s *Script) Get(ctx context.Context) ([]byte, error) {
	if s.File != "" {
		return os.ReadFile(s.File)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.body != nil && time.Since(s.fetched) < time.Hour {
		return s.body, nil
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", s.URL, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if s.body != nil {
			return s.body, nil
		}
		return nil, fmt.Errorf("не скачать install.sh: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil || resp.StatusCode != 200 || !bytes.HasPrefix(body, []byte("#!")) {
		if s.body != nil {
			return s.body, nil
		}
		return nil, fmt.Errorf("не скачать install.sh: HTTP %d", resp.StatusCode)
	}
	s.body, s.fetched = body, time.Now()
	return body, nil
}

var (
	commands = map[string]bool{"probe": true, "apply": true, "stop": true, "restart": true, "logs": true,
		"client-list": true, "client-add": true, "client-conf": true, "client-remove": true, "uninstall": true}
	optKey = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)
)

// Call выполняет команду install.sh. opts - ключи без «--» (значение "" - флаг без значения).
// Значения экранируются; дополнительно их проверяет сам install.sh.
func Call(ctx context.Context, c *sshx.Client, script []byte, cmd string, opts map[string]string, out any) error {
	if !commands[cmd] {
		return fmt.Errorf("rpc: неизвестная команда %q", cmd)
	}
	args := []string{"bash", "-s", "--", cmd}
	keys := make([]string, 0, len(opts))
	for k := range opts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !optKey.MatchString(k) {
			return fmt.Errorf("rpc: недопустимый ключ %q", k)
		}
		if opts[k] == "" {
			args = append(args, "--"+k)
		} else {
			args = append(args, sshx.Quote("--"+k+"="+opts[k]))
		}
	}
	res, err := c.Run(ctx, strings.Join(args, " "), script, cmd != "probe")
	if err != nil {
		return err
	}
	return parse(res, out)
}

// parse ищет последнюю JSON-строку ответа: до неё в stdout может попасть вывод установки пакетов.
func parse(res sshx.Result, out any) error {
	lines := strings.Split(strings.TrimSpace(string(res.Stdout)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(l, `{"proto"`) {
			continue
		}
		var r response
		if err := json.Unmarshal([]byte(l), &r); err != nil {
			return fmt.Errorf("rpc: битый ответ: %w", err)
		}
		if r.Proto != Proto {
			return fmt.Errorf("rpc: install.sh протокола %d, админка понимает %d", r.Proto, Proto)
		}
		if r.Result != "ok" {
			return &Error{Code: r.Code, Msg: r.Msg, Stage: r.Stage, Logs: r.Logs}
		}
		if out != nil && len(r.Data) > 0 {
			return json.Unmarshal(r.Data, out)
		}
		return nil
	}
	msg := strings.TrimSpace(string(res.Stderr))
	switch {
	case strings.Contains(msg, "a password is required") || strings.Contains(msg, "incorrect password"):
		return &Error{Code: "sudo", Msg: "sudo требует пароль - укажите пароль sudo в настройках сервера"}
	case strings.Contains(msg, "bash: command not found") || strings.Contains(msg, "bash: not found"):
		return &Error{Code: "no_bash", Msg: "на сервере нет bash"}
	}
	if len(msg) > 400 {
		msg = msg[len(msg)-400:]
	}
	return &Error{Code: "no_response", Msg: fmt.Sprintf("install.sh не ответил (код %d): %s", res.Code, msg)}
}

// ---- Ответы команд ----

type Config struct {
	Method     string `json:"method"`
	Backend    string `json:"backend"`
	Connect    string `json:"connect"`
	Host       string `json:"host"`
	ListenPort int    `json:"listen_port"`
	WGPort     int    `json:"wg_port"`
	WGNet      string `json:"wg_net"`
	Mode       string `json:"mode"`
	ObfProfile string `json:"obf_profile"`
	ObfKey     string `json:"obf_key"`
	Version    string `json:"version"`
}

type Probe struct {
	Installed bool    `json:"installed"`
	Running   bool    `json:"running"`
	EUID      int     `json:"euid"`
	Arch      string  `json:"arch"`
	Method    string  `json:"method,omitempty"`
	Version   string  `json:"version,omitempty"`
	Config    *Config `json:"config,omitempty"`
}

type Client struct {
	Name     string `json:"name"`
	ClientID string `json:"client_id"`
	Self     bool   `json:"self"`
	IP       string `json:"ip,omitempty"`
	Pub      string `json:"pub,omitempty"`
	HS       int64  `json:"hs,omitempty"` // last handshake, unix
}

type ClientList struct {
	Clients []Client `json:"clients"`
	Share   struct {
		Backend    string `json:"backend"`
		Host       string `json:"host"`
		Port       int    `json:"port"`
		Mode       string `json:"mode"`
		ObfProfile string `json:"obf_profile"`
		ObfKey     string `json:"obf_key"`
	} `json:"share"`
}

type ClientConf struct {
	Client  Client `json:"client"`
	ConfB64 string `json:"conf_b64"`
	Link    string `json:"link"`
}

type Apply struct {
	Owner struct {
		ClientID string `json:"client_id"`
		Pub      string `json:"pub"`
		ConfB64  string `json:"conf_b64"`
		Link     string `json:"link"`
	} `json:"owner"`
	ObfKey       string `json:"obf_key"`
	NeedsRestart bool   `json:"needs_restart"`
}

type Logs struct {
	Lines []string `json:"lines"`
}
