// Package envfile provides a minimal .env loader for local development.
//
// It intentionally does not override variables that are already present in the
// process environment, so real deployment configuration always wins. Keep this
// package development-only: production should inject configuration through the
// process environment, not a checked-in file.
package envfile

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"strings"
)

// Load reads a KEY=VALUE file and sets any variables that are not already set.
// A missing file is not an error; the caller may simply not have one.
func Load(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, value, ok := parseLine(scanner.Text())
		if !ok {
			continue
		}
		if _, present := os.LookupEnv(key); present {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// LoadFirst loads the first existing file among the given candidate paths.
func LoadFirst(paths ...string) error {
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return Load(path)
		}
	}
	return nil
}

func parseLine(line string) (key, value string, ok bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", "", false
	}
	trimmed = strings.TrimPrefix(trimmed, "export ")

	separator := strings.IndexByte(trimmed, '=')
	if separator <= 0 {
		return "", "", false
	}

	key = strings.TrimSpace(trimmed[:separator])
	if key == "" {
		return "", "", false
	}
	value = strings.TrimSpace(trimmed[separator+1:])

	// Strip one layer of matching quotes, if present.
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			value = value[1 : len(value)-1]
		}
	}
	return key, value, true
}
