package secret

import "testing"

func TestRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	b, _ := New(key)
	s, err := b.Seal("hunter2")
	if err != nil || s == "" || s == "hunter2" {
		t.Fatalf("seal: %q %v", s, err)
	}
	if p, err := b.Open(s); err != nil || p != "hunter2" {
		t.Fatalf("open: %q %v", p, err)
	}
	if s2, _ := b.Seal("hunter2"); s2 == s {
		t.Fatal("одинаковый шифртекст: nonce не случайный")
	}
	if e, _ := b.Seal(""); e != "" {
		t.Fatal("пустой секрет должен оставаться пустым")
	}
	other := make([]byte, 32)
	other[0] = 1
	b2, _ := New(other)
	if _, err := b2.Open(s); err == nil {
		t.Fatal("чужой ключ расшифровал")
	}
}
