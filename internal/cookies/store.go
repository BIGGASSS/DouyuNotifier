package cookies

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// Store persists Douyu cookies at its configured path. The default path is
// relative to the process working directory.
type Store struct {
	path string
	out  io.Writer
}

// NewStore creates a cookie store at path and writes diagnostics to out.
func NewStore(path string, out io.Writer) *Store { return &Store{path: path, out: out} }

func (s *Store) printf(format string, args ...any) {
	if s.out != nil {
		fmt.Fprintf(s.out, format, args...)
	}
}

// Load returns an empty cookie set for missing, unreadable, or invalid files.
func (s *Store) Load() map[string]string {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}
	}
	if err != nil {
		s.printf("Warning: Could not load cookies file: %v\n", err)
		return map[string]string{}
	}
	var values map[string]string
	if err := json.Unmarshal(data, &values); err != nil {
		s.printf("Warning: Could not load cookies file: %v\n", err)
		return map[string]string{}
	}
	if values == nil {
		return map[string]string{}
	}
	return values
}

// Save stores cookies with owner-only permissions.
func (s *Store) Save(values map[string]string) {
	data, err := json.MarshalIndent(values, "", "  ")
	if err == nil {
		err = os.WriteFile(s.path, data, 0o600)
	}
	if err == nil {
		err = os.Chmod(s.path, 0o600)
	}
	if err != nil {
		s.printf("Warning: Could not save cookies file: %v\n", err)
		return
	}
	s.printf("✓ Saved cookies to %s\n", s.path)
}

// Parse parses a Cookie header value into name/value pairs.
func Parse(value string) map[string]string {
	cleaned := normalize(value)
	values := map[string]string{}
	for _, pair := range strings.Split(cleaned, ";") {
		pair = strings.TrimSpace(pair)
		separator := strings.IndexByte(pair, '=')
		if pair == "" || separator < 0 {
			continue
		}
		name := strings.TrimSpace(pair[:separator])
		value := strings.TrimSpace(pair[separator+1:])
		if name != "" && value != "" {
			values[name] = value
		}
	}
	return values
}

func normalize(value string) string {
	cleaned := strings.TrimSpace(value)
	if strings.HasPrefix(cleaned, "```") && strings.HasSuffix(cleaned, "```") {
		cleaned = strings.TrimSpace(strings.Trim(cleaned, "`"))
	}
	if len(cleaned) >= len("cookie:") && strings.EqualFold(cleaned[:len("cookie:")], "cookie:") {
		cleaned = strings.TrimSpace(cleaned[len("cookie:"):])
	}
	return strings.ReplaceAll(cleaned, "\n", "; ")
}

// ReadManual supports the original interactive cookie-entry format.
func ReadManual(reader *bufio.Reader, writer io.Writer) map[string]string {
	fmt.Fprintln(writer, "\nEnter cookies (format: name1=value1; name2=value2)")
	fmt.Fprintln(writer, "Or paste individual cookie names and values:")
	fmt.Fprint(writer, "\nCookie string (or press Enter for individual input): ")
	cookieString, _ := reader.ReadString('\n')
	if cookieString = strings.TrimSpace(cookieString); cookieString != "" {
		return Parse(cookieString)
	}
	values := map[string]string{}
	fmt.Fprintln(writer, "\nEnter cookie name and value (empty name to finish):")
	for {
		fmt.Fprint(writer, "Cookie name: ")
		name, readErr := reader.ReadString('\n')
		name = strings.TrimSpace(name)
		if name == "" {
			break
		}
		fmt.Fprint(writer, "Cookie value: ")
		value, valueErr := reader.ReadString('\n')
		if value = strings.TrimSpace(value); value != "" {
			values[name] = value
		}
		if readErr != nil || valueErr != nil {
			break
		}
	}
	return values
}
