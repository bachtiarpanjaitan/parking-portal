package dotenv

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_BasicAndQuoted(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	contents := `# comment line
APP_ENV=development
JWT_SECRET=abc123def456 # hex style (no space before #) MUST be kept
QUOTED="hello world"
SINGLE='value with #hash'
export EXPORTED=yes
EMPTY=
NOT_A_LINE
`
	if err := os.WriteFile(p, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	// Make sure no pre-existing values mess with us.
	for _, k := range []string{"APP_ENV", "JWT_SECRET", "QUOTED", "SINGLE", "EXPORTED", "EMPTY"} {
		_ = os.Unsetenv(k)
	}

	if err := Load(p); err != nil {
		t.Fatalf("Load: %v", err)
	}

	cases := map[string]string{
		"APP_ENV":    "development",
		"JWT_SECRET": "abc123def456",
		"QUOTED":     "hello world",
		"SINGLE":     "value with #hash",
		"EXPORTED":   "yes",
		"EMPTY":      "",
	}
	for k, want := range cases {
		got := os.Getenv(k)
		if got != want {
			t.Errorf("env %s = %q, want %q", k, got, want)
		}
	}

	// Ensure Load never overwrites an already-set value.
	t.Setenv("APP_ENV", "overridden")
	if err := Load(p); err != nil {
		t.Fatalf("Load 2: %v", err)
	}
	if got := os.Getenv("APP_ENV"); got != "overridden" {
		t.Errorf("Load overwrote existing env: got %q", got)
	}
}

func TestLoad_MissingFileIsNotAnError(t *testing.T) {
	if err := Load(filepath.Join(t.TempDir(), "does-not-exist")); err != nil {
		t.Errorf("missing file should be a no-op, got: %v", err)
	}
}

// canonical resolves symlinks (e.g. /var → /private/var on macOS) so two
// paths that refer to the same file compare equal.
func canonical(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

func TestAutoLoad_FindsParentEnv(t *testing.T) {
	root := t.TempDir()
	// Place .env in "root" and cd into root/child.
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("FOO=bar\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(root, "child")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(child); err != nil {
		t.Fatal(err)
	}
	_ = os.Unsetenv("FOO")

	found, err := AutoLoad()
	if err != nil {
		t.Fatalf("AutoLoad: %v", err)
	}
	want, _ := filepath.Abs(filepath.Join(root, ".env"))
	if canonical(found) != canonical(want) {
		t.Errorf("AutoLoad found = %q, want %q", found, want)
	}
	if got := os.Getenv("FOO"); got != "bar" {
		t.Errorf("FOO = %q, want %q", got, "bar")
	}
}
