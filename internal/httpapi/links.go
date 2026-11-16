package httpapi

import (
	"errors"

	"github.com/valyala/fasthttp"

	"github.com/huhhuh78787878-collab/freeturn-admin/internal/link"
)

// linkWithCall вкладывает в ссылку пользователя ссылку на звонок (или убирает её, если callLink пустой).
// Ссылки на звонок админка не хранит: она нужна только в момент выдачи.
func (a *API) linkWithCall(ctx *fasthttp.RequestCtx) {
	var in struct {
		Link     string `json:"link"`
		CallLink string `json:"callLink"`
	}
	if !readJSON(ctx, &in) {
		return
	}
	out, err := link.WithCallLink(in.Link, in.CallLink)
	switch {
	case errors.Is(err, link.ErrBadCallLink):
		failField(ctx, "callLink", err.Error())
	case err != nil:
		failField(ctx, "link", err.Error())
	default:
		writeJSON(ctx, 200, map[string]string{"link": out})
	}
}
