package auth

import "testing"

func TestPassword(t *testing.T) {
	h, err := HashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(h, "correct horse") {
		t.Fatal("верный пароль не принят")
	}
	if CheckPassword(h, "wrong") || CheckPassword("garbage", "correct horse") || CheckPassword("", "") {
		t.Fatal("принят неверный пароль или хэш")
	}
}

func TestTokenHash(t *testing.T) {
	a, b := Token(), Token()
	if a == b || len(a) != 64 || TokenHash(a) == a || TokenHash(a) != TokenHash(a) {
		t.Fatal("токены")
	}
}
