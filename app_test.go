package main

import "testing"

func TestSafeRedirect(t *testing.T) {
	ok := []string{"/painel", "/eventos/novo", "/eventos/3/editar", "/?categoria=teatro&gratis=1"}
	for _, p := range ok {
		if got := safeRedirect(p); got != p {
			t.Errorf("safeRedirect(%q) = %q, queria manter", p, got)
		}
	}
	bad := []string{
		"", "painel", "https://evil.example", "//evil.example", "/\\evil.example",
		"/\tevil.example", "/\n/evil.example", "/ /evil.example", "\x00/x",
		"/painel\r\nX-Header: 1",
	}
	for _, p := range bad {
		if got := safeRedirect(p); got != "/painel" {
			t.Errorf("safeRedirect(%q) = %q, queria /painel", p, got)
		}
	}
}
