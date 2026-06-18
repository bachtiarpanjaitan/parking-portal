// Package dotenv provides a tiny, dependency-free loader for ".env" files.
//
// It is intended to make local `go run` development convenient: the project's
// root .env is auto-discovered (walking up from the current working directory)
// and its values are exported into the process environment unless a variable
// is already set (explicit shell env always wins).
//
// In Docker / CI, the env vars are typically injected by docker-compose or
// the orchestrator, so this loader is a no-op (the file is absent or the
// values are already set).
//
// Supported syntax (a pragmatic subset of the de-facto .env format):
//
//	# comment
//	KEY=value
//	KEY="quoted value with spaces"
//	KEY='single quoted'
//	export KEY=value          # `export` prefix is tolerated
//
// Lines that are blank or only contain a comment are ignored. Values may
// span inline comments when unquoted: everything after a `#` is dropped.
// Quoted values preserve spaces and `#` characters.
package dotenv

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Load reads a .env file and exports its variables into the process env.
// If the file does not exist, it returns nil (no error). Existing env
// variables are NEVER overwritten, so callers can rely on shell env /
// docker-compose env taking precedence.
func Load(path string) error {
	f, err := os.Open(path) // #nosec G304 -- caller-controlled path
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("dotenv: open %s: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Tolerate the shell-style `export ` prefix.
		line = strings.TrimPrefix(line, "export ")
		line = strings.TrimSpace(line)

		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			// Not a KEY=VALUE line; skip silently.
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		val = unquote(val)

		// Never overwrite an explicitly-set env var.
		if _, already := os.LookupEnv(key); already {
			continue
		}
		_ = os.Setenv(key, val)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("dotenv: scan %s: %w", path, err)
	}
	return nil
}

// AutoLoad walks up from the current working directory looking for a `.env`
// file and loads the first one it finds. Returns the absolute path that was
// loaded, or "" if none was found. Errors opening files other than
// "not exists" are returned.
func AutoLoad() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("dotenv: getwd: %w", err)
	}
	dir := cwd
	for {
		candidate := filepath.Join(dir, ".env")
		if _, statErr := os.Stat(candidate); statErr == nil {
			if loadErr := Load(candidate); loadErr != nil {
				return candidate, loadErr
			}
			return candidate, nil
		} else if !os.IsNotExist(statErr) {
			return "", fmt.Errorf("dotenv: stat %s: %w", candidate, statErr)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding a .env.
			return "", nil
		}
		dir = parent
	}
}

// unquote strips a single matching pair of surrounding quotes and trims
// trailing inline comments for unquoted values.
func unquote(v string) string {
	if len(v) >= 2 {
		first, last := v[0], v[len(v)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			return v[1 : len(v)-1]
		}
	}
	// Unquoted: drop trailing `# ...` comment if there is any whitespace
	// before the `#`, so `#` inside a value (e.g. a hex JWT secret) is kept.
	if i := indexUnquotedHash(v); i >= 0 {
		v = v[:i]
	}
	return strings.TrimSpace(v)
}

// indexUnquotedHash returns the index of the first `#` that is preceded by
// whitespace, or -1 if none. This is a best-effort, conservative rule that
// preserves the JWT_SECRET style (no spaces → no truncation).
func indexUnquotedHash(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] != '#' {
			continue
		}
		if i == 0 {
			return -1
		}
		prev := s[i-1]
		if prev == ' ' || prev == '\t' {
			return i
		}
	}
	return -1
}
