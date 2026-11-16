package httpapi

import (
	"errors"
	"net"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/valyala/fasthttp"
	"golang.org/x/crypto/ssh"

	"github.com/huhhuh78787878-collab/freeturn-admin/internal/store"
)

var (
	hostnameRe = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)*[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)
	sshUserRe  = regexp.MustCompile(`^[a-z_][a-z0-9_.-]{0,31}$`)
)

// serverInput - тело POST/PATCH. Секреты необязательны в PATCH: пустая строка = «не менять».
type serverInput struct {
	Name          *string `json:"name"`
	Host          *string `json:"host"`
	SSHPort       *int    `json:"sshPort"`
	SSHUser       *string `json:"sshUser"`
	AuthType      *string `json:"authType"`
	Password      *string `json:"password"`
	PrivateKey    *string `json:"privateKey"`
	KeyPassphrase *string `json:"keyPassphrase"`
	SudoPassword  *string `json:"sudoPassword"`
}

// serverView - то, что видит фронт: без секретов, только флаги «задан».
type serverView struct {
	store.Server
	HasSecret        bool `json:"hasSecret"`
	HasKeyPassphrase bool `json:"hasKeyPassphrase"`
	HasSudoPassword  bool `json:"hasSudoPassword"`
}

func view(v store.Server) serverView {
	return serverView{Server: v, HasSecret: v.SSHSecretEnc != "", HasKeyPassphrase: v.KeyPassphraseEnc != "", HasSudoPassword: v.SudoSecretEnc != ""}
}

func validHost(h string) bool {
	if net.ParseIP(strings.Trim(h, "[]")) != nil {
		return true
	}
	return len(h) <= 253 && hostnameRe.MatchString(h)
}

// apply переносит поля ввода в запись и проверяет итог. creating - все обязательные поля должны прийти.
func (a *API) apply(ctx *fasthttp.RequestCtx, v *store.Server, in serverInput, creating bool) bool {
	if in.Name != nil {
		v.Name = strings.TrimSpace(*in.Name)
	}
	addrChanged := false
	if in.Host != nil {
		h := strings.TrimSpace(*in.Host)
		addrChanged = addrChanged || h != v.Host
		v.Host = h
	}
	if in.SSHPort != nil {
		addrChanged = addrChanged || *in.SSHPort != v.SSHPort
		v.SSHPort = *in.SSHPort
	}
	if in.SSHUser != nil {
		v.SSHUser = strings.TrimSpace(*in.SSHUser)
	}
	if in.AuthType != nil {
		if *in.AuthType != v.AuthType && !creating && in.Password == nil && in.PrivateKey == nil {
			failField(ctx, "authType", "При смене способа входа укажите новый пароль или ключ")
			return false
		}
		v.AuthType = *in.AuthType
	}
	if creating && v.SSHPort == 0 {
		v.SSHPort = 22
	}
	if creating && v.SSHUser == "" {
		v.SSHUser = "root"
	}
	switch {
	case utf8.RuneCountInString(v.Name) < 1 || utf8.RuneCountInString(v.Name) > 64:
		failField(ctx, "name", "Имя: от 1 до 64 символов")
	case !validHost(v.Host):
		failField(ctx, "host", "Адрес: домен или IP")
	case v.SSHPort < 1 || v.SSHPort > 65535:
		failField(ctx, "sshPort", "Порт SSH: 1–65535")
	case !sshUserRe.MatchString(v.SSHUser):
		failField(ctx, "sshUser", "Пользователь SSH: строчная латиница, цифры, _ . -")
	case v.AuthType != "password" && v.AuthType != "key":
		failField(ctx, "authType", "Вход: password или key")
	default:
		return a.applySecrets(ctx, v, in, creating, addrChanged)
	}
	return false
}

func (a *API) applySecrets(ctx *fasthttp.RequestCtx, v *store.Server, in serverInput, creating, addrChanged bool) bool {
	seal := func(plain string, dst *string) bool {
		enc, err := a.Box.Seal(plain)
		if err != nil {
			a.internal(ctx, err)
			return false
		}
		*dst = enc
		return true
	}
	passphrase := ""
	if in.KeyPassphrase != nil {
		passphrase = *in.KeyPassphrase
		if !seal(passphrase, &v.KeyPassphraseEnc) {
			return false
		}
	} else if v.KeyPassphraseEnc != "" {
		p, err := a.Box.Open(v.KeyPassphraseEnc)
		if err != nil {
			a.internal(ctx, err)
			return false
		}
		passphrase = p
	}
	switch v.AuthType {
	case "password":
		if in.Password != nil && *in.Password != "" {
			if len(*in.Password) > 512 {
				failField(ctx, "password", "Слишком длинный пароль")
				return false
			}
			if !seal(*in.Password, &v.SSHSecretEnc) {
				return false
			}
			v.KeyPassphraseEnc = ""
		}
	case "key":
		if in.PrivateKey != nil && strings.TrimSpace(*in.PrivateKey) != "" {
			pem := strings.TrimSpace(*in.PrivateKey) + "\n"
			if err := checkKey(pem, passphrase); err != "" {
				failField(ctx, "privateKey", err)
				return false
			}
			if !seal(pem, &v.SSHSecretEnc) {
				return false
			}
		}
	}
	if creating && v.SSHSecretEnc == "" {
		field := map[string]string{"password": "password", "key": "privateKey"}[v.AuthType]
		failField(ctx, field, "Укажите пароль или ключ SSH")
		return false
	}
	if in.SudoPassword != nil && !seal(*in.SudoPassword, &v.SudoSecretEnc) {
		return false
	}
	if addrChanged && !creating {
		v.HostFingerprint = "" // другой адрес - другой ключ хоста, доверие заново
	}
	return true
}

func checkKey(pem, passphrase string) string {
	var err error
	if passphrase != "" {
		_, err = ssh.ParsePrivateKeyWithPassphrase([]byte(pem), []byte(passphrase))
	} else {
		_, err = ssh.ParsePrivateKey([]byte(pem))
	}
	var missing *ssh.PassphraseMissingError
	switch {
	case err == nil:
		return ""
	case errors.As(err, &missing):
		return "Ключ зашифрован - укажите пароль ключа"
	case passphrase != "":
		return "Не удалось открыть ключ: неверный пароль ключа или формат"
	default:
		return "Нужен приватный ключ OpenSSH или PEM"
	}
}

func (a *API) listServers(ctx *fasthttp.RequestCtx) {
	list, err := a.Store.ListServers(ctx)
	if err != nil {
		a.internal(ctx, err)
		return
	}
	out := make([]serverView, 0, len(list))
	for _, v := range list {
		out = append(out, view(v))
	}
	writeJSON(ctx, 200, out)
}

func (a *API) getServer(ctx *fasthttp.RequestCtx) {
	v, err := a.Store.GetServer(ctx, ctx.UserValue("id").(string))
	if err != nil {
		a.internal(ctx, err)
		return
	}
	writeJSON(ctx, 200, view(v))
}

func (a *API) createServer(ctx *fasthttp.RequestCtx) {
	var in serverInput
	if !readJSON(ctx, &in) {
		return
	}
	var v store.Server
	if in.AuthType == nil {
		v.AuthType = "password"
	}
	if !a.apply(ctx, &v, in, true) {
		return
	}
	v, err := a.Store.CreateServer(ctx, v)
	if err != nil {
		a.internal(ctx, err)
		return
	}
	a.audit(ctx, v.ID, "server.create", map[string]string{"name": v.Name, "host": v.Host}, "ok")
	writeJSON(ctx, 201, view(v))
}

func (a *API) updateServer(ctx *fasthttp.RequestCtx) {
	v, err := a.Store.GetServer(ctx, ctx.UserValue("id").(string))
	if err != nil {
		a.internal(ctx, err)
		return
	}
	var in serverInput
	if !readJSON(ctx, &in) || !a.apply(ctx, &v, in, false) {
		return
	}
	if err := a.Store.UpdateServer(ctx, v); err != nil {
		a.internal(ctx, err)
		return
	}
	a.audit(ctx, v.ID, "server.update", map[string]string{"name": v.Name}, "ok")
	writeJSON(ctx, 200, view(v))
}

func (a *API) deleteServer(ctx *fasthttp.RequestCtx) {
	id := ctx.UserValue("id").(string)
	v, err := a.Store.GetServer(ctx, id)
	if err != nil {
		a.internal(ctx, err)
		return
	}
	if err := a.Store.DeleteServer(ctx, id); err != nil {
		a.internal(ctx, err)
		return
	}
	a.audit(ctx, id, "server.delete", map[string]string{"name": v.Name, "host": v.Host}, "ok")
	writeJSON(ctx, 200, map[string]bool{"ok": true})
}

func (a *API) listAudit(ctx *fasthttp.RequestCtx) {
	limit := ctx.QueryArgs().GetUintOrZero("limit")
	list, err := a.Store.ListAudit(ctx, limit)
	if err != nil {
		a.internal(ctx, err)
		return
	}
	writeJSON(ctx, 200, list)
}
