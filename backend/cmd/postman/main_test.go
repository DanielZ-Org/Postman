package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestDefaultDBPath(t *testing.T) {
	if got := resolveDBPath(func(string) string { return "" }); got != defaultDBPath {
		t.Fatalf("default db path = %q, want %q", got, defaultDBPath)
	}
	if got := resolveDBPath(func(string) string { return "   " }); got != defaultDBPath {
		t.Fatalf("blank db path = %q, want %q", got, defaultDBPath)
	}
	if got := resolveDBPath(func(k string) string {
		if k == "POSTMAN_DB_PATH" {
			return " C:/games/save.db "
		}
		return ""
	}); got != "C:/games/save.db" {
		t.Fatalf("override db path = %q, want C:/games/save.db", got)
	}
}

func TestSetupLogToFileRoutesOutput(t *testing.T) {
	oldOutput := log.Writer()
	defer log.SetOutput(oldOutput)

	logPath := filepath.Join(t.TempDir(), "logs", "postman.log")
	closeLog := setupLogToFile(logPath)
	log.Print("marker-line-for-log-file")
	closeLog()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if !strings.Contains(string(data), "marker-line-for-log-file") {
		t.Errorf("log file does not contain the marker: %s", data)
	}
}

func TestSetupLogToFileFallsBackWhenUnusable(t *testing.T) {
	oldOutput := log.Writer()
	defer log.SetOutput(oldOutput)

	// A regular file where a directory must go: creating the parent fails and
	// logging must fall back to stderr instead of crashing.
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed blocker: %v", err)
	}
	closeLog := setupLogToFile(filepath.Join(blocker, "sub", "postman.log"))
	if closeLog == nil {
		t.Fatal("setupLogToFile returned nil closer")
	}
	log.Print("still-routes-to-stderr")
	closeLog()
}
