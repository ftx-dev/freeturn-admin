package httpapi

import (
	"regexp"
	"unicode/utf8"

	"github.com/valyala/fasthttp"

	"github.com/huhhuh78787878-collab/freeturn-admin/internal/auth"
	"github.com/huhhuh78787878-collab/freeturn-admin/internal/store"
)

var loginRe = regexp.MustCompile(`^[a-zA-Z0-9_.-]{3,32}$`)

func hashToken(t string) string { return auth.TokenHash(t) }

// dummyHash: проверка пароля для несуществующего логина занимает столько же времени.
var dummyHash, _ = auth.HashPassword("dummy-password-for-timing")

type credentials struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func validateCredentials(ctx *fasthttp.RequestCtx, c credentials) bool {
	if !loginRe.MatchString(c.Login) {
		failField(ctx, "login", "Логин: 3–32 символа, латиница, цифры, . _ -")
		return false
	}
	if n := utf8.RuneCountInString(c.Password); n < 10 || n > 200 {
		failField(ctx, "password", "Пароль: от 10 символов")
		return false
	}
	return true
}

func (a *API) authState(ctx *fasthttp.RequestCtx) {
	n, err := a.Store.CountAdmins(ctx)
	if err != nil {
		a.internal(ctx, err)
		return
	}
	writeJSON(ctx, 200, map[string]bool{"needsSetup": n == 0})
}

// setup создаёт первого администратора - только пока их нет.
func (a *API) setup(ctx *fasthttp.RequestCtx) {
	if !a.loginLimit.Allow("setup:" + a.clientIP(ctx)) {
		fail(ctx, 429, "rate_limited", "Слишком много попыток, подождите")
		return
	}
	var c credentials
	if !readJSON(ctx, &c) || !validateCredentials(ctx, c) {
		return
	}
	if n, err := a.Store.CountAdmins(ctx); err != nil || n > 0 {
		fail(ctx, 409, "already_setup", "Администратор уже создан")
		return
	}
	hash, err := auth.HashPassword(c.Password)
	if err != nil {
		a.internal(ctx, err)
		return
	}
	admin, err := a.Store.CreateAdmin(ctx, c.Login, hash, "admin")
	if err != nil {
		fail(ctx, 409, "already_setup", "Администратор уже создан")
		return
	}
	ctx.SetUserValue(ctxAdmin, admin)
	a.audit(ctx, "", "setup", map[string]string{"login": c.Login}, "ok")
	a.startSession(ctx, admin)
}

func (a *API) login(ctx *fasthttp.RequestCtx) {
	var c credentials
	if !readJSON(ctx, &c) {
		return
	}
	ip := a.clientIP(ctx)
	if !a.loginLimit.Allow("ip:"+ip) || !a.loginLimit.Allow("login:"+c.Login) {
		fail(ctx, 429, "rate_limited", "Слишком много попыток, подождите 15 минут")
		return
	}
	admin, err := a.Store.AdminByLogin(ctx, c.Login)
	hash := admin.PasswordHash
	if err != nil {
		hash = dummyHash
	}
	if !auth.CheckPassword(hash, c.Password) || err != nil {
		a.audit(ctx, "", "login", map[string]string{"login": c.Login, "ip": ip}, "fail")
		fail(ctx, 401, "bad_credentials", "Неверный логин или пароль")
		return
	}
	a.loginLimit.Reset("login:" + c.Login)
	a.Store.TouchLogin(ctx, admin.ID)
	ctx.SetUserValue(ctxAdmin, admin)
	a.audit(ctx, "", "login", map[string]string{"ip": ip}, "ok")
	a.startSession(ctx, admin)
}

func (a *API) startSession(ctx *fasthttp.RequestCtx, admin store.Admin) {
	token, csrf := auth.Token(), auth.Token()
	if err := a.Store.CreateSession(ctx, hashToken(token), admin.ID, csrf, sessionTTL, a.clientIP(ctx),
		truncate(string(ctx.UserAgent()), 300)); err != nil {
		a.internal(ctx, err)
		return
	}
	c := fasthttp.AcquireCookie()
	defer fasthttp.ReleaseCookie(c)
	c.SetKey(sessionCookie)
	c.SetValue(token)
	c.SetPath("/")
	c.SetHTTPOnly(true)
	c.SetSecure(a.SecureCookies)
	c.SetSameSite(fasthttp.CookieSameSiteStrictMode)
	c.SetMaxAge(int(sessionTTL))
	ctx.Response.Header.SetCookie(c)
	writeJSON(ctx, 200, meResponse{Login: admin.Login, Role: admin.Role, CSRF: csrf})
}

func (a *API) logout(ctx *fasthttp.RequestCtx) {
	se := ctx.UserValue(ctxSession).(store.Session)
	a.Store.DeleteSession(ctx, se.ID)
	ctx.Response.Header.DelClientCookie(sessionCookie)
	writeJSON(ctx, 200, map[string]bool{"ok": true})
}

type meResponse struct {
	Login string `json:"login"`
	Role  string `json:"role"`
	CSRF  string `json:"csrf"`
}

func (a *API) me(ctx *fasthttp.RequestCtx) {
	admin := adminOf(ctx)
	writeJSON(ctx, 200, meResponse{Login: admin.Login, Role: admin.Role, CSRF: ctx.UserValue(ctxSession).(store.Session).CSRF})
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
