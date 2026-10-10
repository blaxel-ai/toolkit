package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadSecretValue_flagWins(t *testing.T) {
	v, err := readSecretValue(true, "abc", "", "", strings.NewReader("ignored"))
	if err != nil || v != "abc" {
		t.Fatalf("got %q, %v", v, err)
	}
}

func TestReadSecretValue_fromEnv(t *testing.T) {
	t.Setenv("BL_TEST_SECRET", "from-env")
	v, err := readSecretValue(false, "", "BL_TEST_SECRET", "", strings.NewReader(""))
	if err != nil || v != "from-env" {
		t.Fatalf("got %q, %v", v, err)
	}
}

func TestReadSecretValue_fromEnvUnset(t *testing.T) {
	if _, err := readSecretValue(false, "", "BL_TEST_SECRET_UNSET", "", strings.NewReader("")); err == nil {
		t.Fatal("expected error for unset env var")
	}
}

func TestReadSecretValue_fromFileTrimsNewline(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.txt")
	if err := os.WriteFile(p, []byte("file-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	v, err := readSecretValue(false, "", "", p, strings.NewReader(""))
	if err != nil || v != "file-value" {
		t.Fatalf("got %q, %v", v, err)
	}
}

func TestReadSecretValue_stdinTrimsNewline(t *testing.T) {
	v, err := readSecretValue(false, "", "", "", strings.NewReader("piped\r\n"))
	if err != nil || v != "piped" {
		t.Fatalf("got %q, %v", v, err)
	}
}

func TestReadSecretValue_multipleSourcesRejected(t *testing.T) {
	if _, err := readSecretValue(true, "a", "B", "", strings.NewReader("")); err == nil {
		t.Fatal("expected error when two sources are given")
	}
}

func TestReadSecretValue_emptyRejected(t *testing.T) {
	if _, err := readSecretValue(false, "", "", "", strings.NewReader("\n")); err == nil {
		t.Fatal("expected error for empty value")
	}
}

func TestReadSecretValue_emptyValueFlagDoesNotFallBackToStdin(t *testing.T) {
	if _, err := readSecretValue(true, "", "", "", strings.NewReader("piped")); err == nil {
		t.Fatal("expected error: --value '' must not read stdin")
	}
}

func TestReadSecretValue_emptyValueFlagWithFileRejected(t *testing.T) {
	f := filepath.Join(t.TempDir(), "s")
	if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readSecretValue(true, "", "", f, strings.NewReader("")); err == nil {
		t.Fatal("expected error when --value and --from-file are both given")
	}
}
