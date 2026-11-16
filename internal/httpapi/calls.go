package httpapi

import (
	"encoding/json"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/valyala/fasthttp"

	"github.com/huhhuh78787878-collab/freeturn-admin/internal/link"
)

// Call - ссылка на звонок сервера с необязательной подписью; предлагается при выдаче доступа.
type Call struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

const maxCalls = 50

func (a *API) loadCalls(ctx *fasthttp.RequestCtx, id string) ([]Call, bool) {
	enc, err := a.Store.Calls(ctx, id)
	if err != nil {
		a.internal(ctx, err)
		return nil, false
	}
	plain, err := a.Box.Open(enc)
	if err != nil {
		a.internal(ctx, err)
		return nil, false
	}
	calls := []Call{}
	if plain != "" {
		if err := json.Unmarshal([]byte(plain), &calls); err != nil {
			a.internal(ctx, err)
			return nil, false
		}
	}
	return calls, true
}

func (a *API) getCalls(ctx *fasthttp.RequestCtx) {
	if calls, ok := a.loadCalls(ctx, ctx.UserValue("id").(string)); ok {
		writeJSON(ctx, 200, map[string]any{"calls": calls})
	}
}

func (a *API) putCalls(ctx *fasthttp.RequestCtx) {
	var in struct {
		Calls []Call `json:"calls"`
	}
	if !readJSON(ctx, &in) {
		return
	}
	if len(in.Calls) > maxCalls {
		failField(ctx, "calls", "Не больше 50 звонков")
		return
	}
	seen := map[string]bool{}
	clean := make([]Call, 0, len(in.Calls))
	for i, c := range in.Calls {
		c.Label, c.URL = strings.TrimSpace(c.Label), strings.TrimSpace(c.URL)
		if c.URL == "" {
			continue
		}
		// Та же проверка, что при вложении в ссылку: только https.
		if _, err := link.WithCallLink("freeturn://e30", c.URL); err != nil {
			failField(ctx, "calls."+strconv.Itoa(i), err.Error())
			return
		}
		if utf8.RuneCountInString(c.Label) > 64 {
			failField(ctx, "calls."+strconv.Itoa(i), "Подпись: до 64 символов")
			return
		}
		if !seen[c.URL] {
			seen[c.URL] = true
			clean = append(clean, c)
		}
	}
	id := ctx.UserValue("id").(string)
	b, _ := json.Marshal(clean)
	enc, err := a.Box.Seal(string(b))
	if err != nil {
		a.internal(ctx, err)
		return
	}
	if err := a.Store.SetCalls(ctx, id, enc); err != nil {
		a.internal(ctx, err)
		return
	}
	a.audit(ctx, id, "server.calls", map[string]int{"count": len(clean)}, "ok")
	writeJSON(ctx, 200, map[string]any{"calls": clean})
}
