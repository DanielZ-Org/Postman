package main

import "testing"

func TestDefaultListenAddr(t *testing.T) {
	got := resolveListenAddr(func(string) string { return "" })
	if got != defaultListenAddr {
		t.Fatalf("default listen addr = %q, want %q", got, defaultListenAddr)
	}
}

func TestBackendAddrOverride(t *testing.T) {
	getenv := func(k string) string {
		if k == "BACKEND_ADDR" {
			return "127.0.0.1:9090"
		}
		return ""
	}

	got := resolveListenAddr(getenv)
	if got != "127.0.0.1:9090" {
		t.Fatalf("override listen addr = %q, want %q", got, "127.0.0.1:9090")
	}
}

func TestBlankBackendAddrFallsBackToDefault(t *testing.T) {
	getenv := func(k string) string {
		if k == "BACKEND_ADDR" {
			return "   "
		}
		return ""
	}

	got := resolveListenAddr(getenv)
	if got != defaultListenAddr {
		t.Fatalf("blank override listen addr = %q, want %q", got, defaultListenAddr)
	}
}

func TestIsAllInterfaceBind(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1:8080": false,
		"localhost:8080": false,
		"0.0.0.0:8080":   true,
		":8080":          true,
		"[::]:8080":      true,
	}

	for addr, want := range cases {
		if got := isAllInterfaceBind(addr); got != want {
			t.Errorf("isAllInterfaceBind(%q) = %v, want %v", addr, got, want)
		}
	}
}
