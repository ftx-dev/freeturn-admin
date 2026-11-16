package httpapi

import (
	"github.com/valyala/fasthttp"

	"github.com/huhhuh78787878-collab/freeturn-admin/internal/auth"
	"github.com/huhhuh78787878-collab/freeturn-admin/internal/store"
)

// Роли учёток админки:
//
//	root     - владелец, первый администратор; его нельзя удалить или понизить;
//	admin    - всё, включая добавление и удаление учёток (кроме владельца);
//	operator - всё с серверами и их пользователями, учётки админки только просматривает.
func canManageAdmins(role string) bool { return role == "root" || role == "admin" }

// managers пропускает только тех, кто может менять учётки админки.
func (a *API) managers(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return a.authed(func(ctx *fasthttp.RequestCtx) {
		if !canManageAdmins(adminOf(ctx).Role) {
			fail(ctx, 403, "forbidden", "Управлять учётками может только администратор")
			return
		}
		next(ctx)
	})
}

func (a *API) listAdmins(ctx *fasthttp.RequestCtx) {
	list, err := a.Store.ListAdmins(ctx)
	if err != nil {
		a.internal(ctx, err)
		return
	}
	writeJSON(ctx, 200, list)
}

type adminInput struct {
	Login    string `json:"login"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

func validRole(r string) bool { return r == "admin" || r == "operator" }

func (a *API) createAdmin(ctx *fasthttp.RequestCtx) {
	var in adminInput
	if !readJSON(ctx, &in) || !validateCredentials(ctx, credentials{in.Login, in.Password}) {
		return
	}
	if !validRole(in.Role) {
		failField(ctx, "role", "Роль: admin или operator")
		return
	}
	if _, err := a.Store.AdminByLogin(ctx, in.Login); err == nil {
		failField(ctx, "login", "Такой логин уже есть")
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		a.internal(ctx, err)
		return
	}
	created, err := a.Store.CreateAdmin(ctx, in.Login, hash, in.Role)
	if err != nil {
		failField(ctx, "login", "Такой логин уже есть")
		return
	}
	a.audit(ctx, "", "admin.create", map[string]string{"login": in.Login, "role": in.Role}, "ok")
	writeJSON(ctx, 201, created)
}

// target - учётка из пути; владельца и себя через эти ручки не меняют.
func (a *API) targetAdmin(ctx *fasthttp.RequestCtx) (store.Admin, bool) {
	t, err := a.Store.AdminByID(ctx, ctx.UserValue("id").(string))
	if err != nil {
		a.internal(ctx, err)
		return t, false
	}
	switch {
	case t.Role == "root":
		fail(ctx, 403, "forbidden", "Владельца админки нельзя удалить или изменить")
		return t, false
	case t.ID == adminOf(ctx).ID:
		fail(ctx, 403, "forbidden", "Свою учётку так не меняют")
		return t, false
	}
	return t, true
}

func (a *API) updateAdmin(ctx *fasthttp.RequestCtx) {
	var in struct {
		Role     *string `json:"role"`
		Password *string `json:"password"`
	}
	if !readJSON(ctx, &in) {
		return
	}
	t, ok := a.targetAdmin(ctx)
	if !ok {
		return
	}
	if in.Role != nil {
		if !validRole(*in.Role) {
			failField(ctx, "role", "Роль: admin или operator")
			return
		}
		if err := a.Store.SetAdminRole(ctx, t.ID, *in.Role); err != nil {
			a.internal(ctx, err)
			return
		}
		t.Role = *in.Role
	}
	if in.Password != nil {
		if !validateCredentials(ctx, credentials{t.Login, *in.Password}) {
			return
		}
		hash, err := auth.HashPassword(*in.Password)
		if err != nil {
			a.internal(ctx, err)
			return
		}
		if err := a.Store.SetAdminPassword(ctx, t.ID, hash); err != nil {
			a.internal(ctx, err)
			return
		}
		a.Store.DeleteSessionsExcept(ctx, t.ID, "") // новый пароль - старые входы больше не действуют
	}
	a.audit(ctx, "", "admin.update", map[string]any{"login": t.Login, "role": t.Role, "password": in.Password != nil}, "ok")
	writeJSON(ctx, 200, t)
}

func (a *API) deleteAdmin(ctx *fasthttp.RequestCtx) {
	t, ok := a.targetAdmin(ctx)
	if !ok {
		return
	}
	if err := a.Store.DeleteAdmin(ctx, t.ID); err != nil {
		a.internal(ctx, err)
		return
	}
	a.audit(ctx, "", "admin.delete", map[string]string{"login": t.Login}, "ok")
	writeJSON(ctx, 200, map[string]bool{"ok": true})
}

// changeOwnPassword - любой может сменить свой пароль, зная текущий.
func (a *API) changeOwnPassword(ctx *fasthttp.RequestCtx) {
	var in struct {
		Current string `json:"current"`
		New     string `json:"new"`
	}
	if !readJSON(ctx, &in) {
		return
	}
	me := adminOf(ctx)
	if !auth.CheckPassword(me.PasswordHash, in.Current) {
		failField(ctx, "current", "Неверный текущий пароль")
		return
	}
	if !validateCredentials(ctx, credentials{me.Login, in.New}) {
		return
	}
	hash, err := auth.HashPassword(in.New)
	if err != nil {
		a.internal(ctx, err)
		return
	}
	if err := a.Store.SetAdminPassword(ctx, me.ID, hash); err != nil {
		a.internal(ctx, err)
		return
	}
	a.Store.DeleteSessionsExcept(ctx, me.ID, ctx.UserValue(ctxSession).(store.Session).ID)
	a.audit(ctx, "", "admin.password", nil, "ok")
	writeJSON(ctx, 200, map[string]bool{"ok": true})
}
