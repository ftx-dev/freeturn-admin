package link

import (
	"encoding/base64"
	"strings"
	"testing"
)

func mk(json string) string { return scheme + base64.RawURLEncoding.EncodeToString([]byte(json)) }

func payload(t *testing.T, l string) string {
	t.Helper()
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(l, scheme))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestWithCallLink(t *testing.T) {
	call := "https://vk.com/call/join/abc_DEF-123?x=1&y=2"
	base := `{"v":1,"provider":"vk","peer":"1.2.3.4:56000","obf":"rtpopus3","key":"ab","cid":"c1","kcp":{"nodelay":1},"name":"bob"}`
	got, err := WithCallLink(mk(base), call)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"v":1,"provider":"vk","peer":"1.2.3.4:56000","obf":"rtpopus3","key":"ab","cid":"c1","kcp":{"nodelay":1},"name":"bob","vk":"https://vk.com/call/join/abc_DEF-123?x=1&y=2"}`
	if p := payload(t, got); p != want {
		t.Fatalf("\n got %s\nwant %s", p, want)
	}

	// Повторное вложение заменяет, а не дублирует; vk встаёт перед wg; пустая строка убирает.
	withWG := mk(`{"v":1,"provider":"vk","peer":"p","name":"a","vk":"https://old.example/x","wg":"[Interface]"}`)
	got, _ = WithCallLink(withWG, call)
	if p := payload(t, got); p != `{"v":1,"provider":"vk","peer":"p","name":"a","vk":"`+call+`","wg":"[Interface]"}` {
		t.Fatal(p)
	}
	got, _ = WithCallLink(withWG, "")
	if p := payload(t, got); p != `{"v":1,"provider":"vk","peer":"p","name":"a","wg":"[Interface]"}` {
		t.Fatal(p)
	}

	// Ссылка, совпадающая байт в байт с macOS-клиентом (FreeturnLink.encode в freeturn-macos).
	got, _ = WithCallLink(mk(`{"v":1,"provider":"vk","peer":"1.2.3.4:56000","name":"Мой"}`), "https://vk.com/call/join/q")
	if got != mk(`{"v":1,"provider":"vk","peer":"1.2.3.4:56000","name":"Мой","vk":"https://vk.com/call/join/q"}`) {
		t.Fatal("не совпадает с клиентами:", payload(t, got))
	}

	for _, bad := range []string{"http://vk.com/x", "javascript:alert(1)", "vk.com/call", "https://"} {
		if _, err := WithCallLink(mk(base), bad); err != ErrBadCallLink {
			t.Errorf("%q: %v", bad, err)
		}
	}
	if _, err := WithCallLink("https://not-a-link", call); err != ErrBadLink {
		t.Fatal(err)
	}
}
