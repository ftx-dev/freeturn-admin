// Package httpapi - REST API админки на fasthttp и раздача собранного фронта.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"mime"
	"path"
	"strings"
	"time"

	"github.com/fasthttp/router"
	"github.com/valyala/fasthttp"

	"github.com/huhhuh78787878-collab/freeturn-admin/internal/rpc"
	"github.com/huhhuh78787878-collab/freeturn-admin/internal/secret"
	"github.com/huhhuh78787878-collab/freeturn-admin/internal/store"
)

type API struct {
	Store         *store.Store
	Box           *secret.Box
	Web           fs.FS // собранный фронт (web/dist); nil - только API
	SecureCookies bool
	TrustProxy    bool
	Log           *slog.Logger
	Script        *rpc.Script // install.sh ядра

	loginLimit *limiter
	jobs       jobs
	health     healthCache
}

const (
	sessionCookie = "fta_session"
	sessionTTL    = int64(7 * 24 * 3600)
	ctxAdmin      = "admin"
	ctxSession    = "session"
)

func (a *API) Handler() fasthttp.RequestHandler {
	a.loginLimit = newLimiter(10, 15*time.Minute)
	a.jobs = jobs{byID: map[string]*Job{}, busy: map[string]string{}}
	r := router.New()
	r.GET("/api/health", func(ctx *fasthttp.RequestCtx) { writeJSON(ctx, 200, map[string]string{"status": "ok"}) })

	r.GET("/api/auth/state", a.authState)
	r.POST("/api/auth/setup", a.setup)
	r.POST("/api/auth/login", a.login)
	r.POST("/api/auth/logout", a.authed(a.logout))
	r.GET("/api/auth/me", a.authed(a.me))

	r.GET("/api/servers", a.authed(a.listServers))
	r.POST("/api/servers", a.authed(a.createServer))
	r.GET("/api/servers/health", a.authed(a.serversHealth))
	r.GET("/api/servers/{id}", a.authed(a.getServer))
	r.PATCH("/api/servers/{id}", a.authed(a.updateServer))
	r.DELETE("/api/servers/{id}", a.authed(a.deleteServer))

	r.POST("/api/servers/{id}/ssh/test", a.authed(a.sshTest))
	r.POST("/api/servers/{id}/ssh/trust", a.authed(a.sshTrust))
	r.GET("/api/servers/{id}/status", a.authed(a.status))
	r.GET("/api/servers/{id}/logs", a.authed(a.serverLogs))
	r.GET("/api/servers/{id}/users", a.authed(a.listUsers))
	r.POST("/api/servers/{id}/users", a.authed(a.addUser))
	r.GET("/api/servers/{id}/users/{name}", a.authed(a.getUser))
	r.DELETE("/api/servers/{id}/users/{name}", a.authed(a.removeUser))
	r.POST("/api/servers/{id}/apply", a.authed(a.applyServer))
	r.POST("/api/servers/{id}/stop", a.authed(a.simple("stop", "server.stop")))
	r.POST("/api/servers/{id}/restart", a.authed(a.simple("restart", "server.restart")))
	r.POST("/api/servers/{id}/uninstall", a.authed(a.uninstall))
	r.GET("/api/jobs/{id}", a.authed(a.getJob))
	r.GET("/api/defaults", a.authed(func(rc *fasthttp.RequestCtx) {
		writeJSON(rc, 200, map[string]any{"listenPort": RandomListenPort(), "obfKey": NewObfKey()})
	}))

	r.GET("/api/audit", a.authed(a.listAudit))

	r.GET("/api/admins", a.authed(a.listAdmins))
	r.POST("/api/admins", a.managers(a.createAdmin))
	r.PATCH("/api/admins/{id}", a.managers(a.updateAdmin))
	r.DELETE("/api/admins/{id}", a.managers(a.deleteAdmin))
	r.POST("/api/auth/password", a.authed(a.changeOwnPassword))

	r.NotFound = a.static
	return securityHeaders(r.Handler)
}

func securityHeaders(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		h := &ctx.Response.Header
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		next(ctx)
	}
}

// authed: живая сессия из cookie и, для изменяющих запросов, заголовок X-CSRF-Token той же сессии.
func (a *API) authed(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		token := string(ctx.Request.Header.Cookie(sessionCookie))
		if token == "" {
			fail(ctx, 401, "unauthorized", "Нужен вход")
			return
		}
		se, err := a.Store.SessionByHash(ctx, hashToken(token))
		if err != nil {
			fail(ctx, 401, "unauthorized", "Сессия истекла, войдите снова")
			return
		}
		if !ctx.IsGet() && !ctx.IsHead() && string(ctx.Request.Header.Peek("X-CSRF-Token")) != se.CSRF {
			fail(ctx, 403, "csrf", "Неверный CSRF-токен, обновите страницу")
			return
		}
		admin, err := a.Store.AdminByID(ctx, se.AdminID)
		if err != nil {
			fail(ctx, 401, "unauthorized", "Учётная запись не найдена")
			return
		}
		ctx.SetUserValue(ctxAdmin, admin)
		ctx.SetUserValue(ctxSession, se)
		next(ctx)
	}
}

func adminOf(ctx *fasthttp.RequestCtx) store.Admin { return ctx.UserValue(ctxAdmin).(store.Admin) }

func (a *API) clientIP(ctx *fasthttp.RequestCtx) string {
	if a.TrustProxy {
		if xff := string(ctx.Request.Header.Peek("X-Forwarded-For")); xff != "" {
			return strings.TrimSpace(strings.Split(xff, ",")[0])
		}
	}
	return ctx.RemoteIP().String()
}

func (a *API) audit(ctx *fasthttp.RequestCtx, serverID, action string, details any, result string) {
	b, _ := json.Marshal(details)
	adminID := ""
	if v, ok := ctx.UserValue(ctxAdmin).(store.Admin); ok {
		adminID = v.ID
	}
	if err := a.Store.Audit(context.Background(), adminID, serverID, action, string(b), result); err != nil {
		a.Log.Error("audit", "err", err)
	}
}

// static отдаёт файлы фронта; неизвестный путь вне /api - index.html (маршрутизация на стороне браузера).
func (a *API) static(ctx *fasthttp.RequestCtx) {
	p := string(ctx.Path())
	if a.Web == nil || strings.HasPrefix(p, "/api/") {
		fail(ctx, 404, "not_found", "Не найдено")
		return
	}
	name := strings.TrimPrefix(path.Clean(p), "/")
	data, err := fs.ReadFile(a.Web, name)
	if name == "" || err != nil {
		name = "index.html"
		if data, err = fs.ReadFile(a.Web, name); err != nil {
			fail(ctx, 404, "not_found", "Фронт не собран")
			return
		}
	}
	if ct := mime.TypeByExtension(path.Ext(name)); ct != "" {
		ctx.SetContentType(ct)
	}
	if strings.HasPrefix(name, "assets/") {
		ctx.Response.Header.Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		ctx.Response.Header.Set("Cache-Control", "no-cache")
	}
	ctx.SetBody(data)
}

func readJSON(ctx *fasthttp.RequestCtx, v any) bool {
	dec := json.NewDecoder(strings.NewReader(string(ctx.PostBody())))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		fail(ctx, 400, "bad_json", "Некорректный JSON: "+err.Error())
		return false
	}
	return true
}

func writeJSON(ctx *fasthttp.RequestCtx, status int, v any) {
	ctx.SetStatusCode(status)
	ctx.SetContentType("application/json; charset=utf-8")
	ctx.Response.Header.Set("Cache-Control", "no-store")
	b, _ := json.Marshal(v)
	ctx.SetBody(b)
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

func fail(ctx *fasthttp.RequestCtx, status int, code, msg string) {
	writeJSON(ctx, status, map[string]apiError{"error": {Code: code, Message: msg}})
}

func failField(ctx *fasthttp.RequestCtx, field, msg string) {
	writeJSON(ctx, 422, map[string]apiError{"error": {Code: "invalid", Message: msg, Field: field}})
}

func (a *API) internal(ctx *fasthttp.RequestCtx, err error) {
	if errors.Is(err, store.ErrNotFound) {
		fail(ctx, 404, "not_found", "Не найдено")
		return
	}
	a.Log.Error("api", "path", string(ctx.Path()), "err", err)
	fail(ctx, 500, "internal", "Внутренняя ошибка")
}
