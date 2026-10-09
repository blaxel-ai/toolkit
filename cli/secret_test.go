package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadSecretValue_flagWins(t *testing.T) {
	v, err := readSecretValue("abc", "", "", strings.NewReader("ignored"))
	if err != nil || v != "abc" {
		t.Fatalf("got %q, %v", v, err)
	}
}

func TestReadSecretValue_fromEnv(t *testing.T) {
	t.Setenv("BL_TEST_SECRET", "from-env")
	v, err := readSecretValue("", "BL_TEST_SECRET", "", strings.NewReader(""))
	if err != nil || v != "from-env" {
		t.Fatalf("got %q, %v", v, err)
	}
}

func TestReadSecretValue_fromEnvUnset(t *testing.T) {
	if _, err := readSecretValue("", "BL_TEST_SECRET_UNSET", "", strings.NewReader("")); err == nil {
		t.Fatal("expected error for unset env var")
	}
}

func TestReadSecretValue_fromFileTrimsNewline(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.txt")
	if err := os.WriteFile(p, []byte("file-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	v, err := readSecretValue("", "", p, strings.NewReader(""))
	if err != nil || v != "file-value" {
		t.Fatalf("got %q, %v", v, err)
	}
}

func TestReadSecretValue_stdinTrimsNewline(t *testing.T) {
	v, err := readSecretValue("", "", "", strings.NewReader("piped\r\n"))
	if err != nil || v != "piped" {
		t.Fatalf("got %q, %v", v, err)
	}
}

func TestReadSecretValue_multipleSourcesRejected(t *testing.T) {
	if _, err := readSecretValue("a", "B", "", strings.NewReader("")); err == nil {
		t.Fatal("expected error when two sources are given")
	}
}

func TestReadSecretValue_emptyRejected(t *testing.T) {
	if _, err := readSecretValue("", "", "", strings.NewReader("\n")); err == nil {
		t.Fatal("expected error for empty value")
	}
}
