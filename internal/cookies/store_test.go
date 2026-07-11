package cookies

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name, input string
		want        map[string]string
	}{
		{"plain", "acf_uid=123; dy_did=abc; malformed", map[string]string{"acf_uid": "123", "dy_did": "abc"}},
		{"code block", "```\nCookie: acf_uid=123; dy_did=abc\n```", map[string]string{"acf_uid": "123", "dy_did": "abc"}},
		{"multiline and equals", "Cookie: token=a=b=c\nempty=; =value\nname=value", map[string]string{"token": "a=b=c", "name": "value"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Parse(tt.input); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Parse() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestStoreSaveLoadAndPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cookies.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(path, ioDiscard{})
	want := map[string]string{"acf_uid": "123", "dy_did": "abc"}
	store.Save(want)
	if got := store.Load(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Load() = %#v", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 600", info.Mode().Perm())
	}
}

func TestStoreInvalidJSONReturnsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cookies.json")
	if err := os.WriteFile(path, []byte("bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := NewStore(path, ioDiscard{}).Load(); len(got) != 0 {
		t.Fatalf("Load() = %#v", got)
	}
}

func TestReadManualIndividualValues(t *testing.T) {
	var output bytes.Buffer
	got := ReadManual(bufio.NewReader(strings.NewReader("\nacf_uid\n123\ndy_did\nabc\n\n")), &output)
	want := map[string]string{"acf_uid": "123", "dy_did": "abc"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadManual() = %#v", got)
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }
