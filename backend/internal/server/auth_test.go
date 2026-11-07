package server

import (
	"strings"
	"testing"

	"ebku/internal/store"
)

func TestTemplatesParse(t *testing.T) {
	if _, err := New(nil, ""); err != nil {
		t.Fatalf("template tidak bisa diparse: %v", err)
	}
}

func TestSafeNext(t *testing.T) {
	cases := map[string]string{
		"":                    "/",
		"/bantuan/1/bku":      "/bantuan/1/bku",
		"//evil.example":      "/",
		"https://evil.com":    "/",
		"javascript:alert(1)": "/",
		"/\\evil.example":     "/",
	}
	for in, want := range cases {
		if got := safeNext(in); got != want {
			t.Errorf("safeNext(%q)=%q, want %q", in, got, want)
		}
	}
}

type testPage struct {
	baseView
	Title string
}

func TestInjectBaseFields(t *testing.T) {
	u := &store.User{ID: 1, Username: "admin", Role: store.RoleAdmin}
	p := testPage{}
	out := injectBaseFields(p, u, "csrf-token", "cari").(testPage)
	if out.User != u || out.CSRF != "csrf-token" || out.SearchQuery != "cari" {
		t.Fatalf("base fields tidak terinject: %+v", out.baseView)
	}
	if !out.CanWrite() || !out.IsAdmin() {
		t.Fatal("role admin harus CanWrite dan IsAdmin")
	}
}

func TestCSRFHeaderConstant(t *testing.T) {
	if csrfHeaderName != "X-CSRF-Token" || !strings.HasPrefix(sessionCookieName, "ebku_") {
		t.Fatal("nama cookie/header tidak sesuai")
	}
}
