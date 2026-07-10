package main

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseCookieStringPlain(t *testing.T) {
	got := parseCookieString("acf_uid=123; dy_did=abc; malformed")
	want := map[string]string{"acf_uid": "123", "dy_did": "abc"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseCookieString() = %#v, want %#v", got, want)
	}
}

func TestParseCookieStringTelegramCodeBlock(t *testing.T) {
	got := parseCookieString("```\nCookie: acf_uid=123; dy_did=abc\n```")
	want := map[string]string{"acf_uid": "123", "dy_did": "abc"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseCookieString() = %#v, want %#v", got, want)
	}
}

func TestParseCookieStringMultilineAndEqualsInValue(t *testing.T) {
	got := parseCookieString("Cookie: token=a=b=c\nempty=; =value\nname=value")
	want := map[string]string{"token": "a=b=c", "name": "value"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseCookieString() = %#v, want %#v", got, want)
	}
}

func TestSaveAndLoadCookies(t *testing.T) {
	originalFile := cookiesFile
	cookiesFile = filepath.Join(t.TempDir(), "cookies.json")
	t.Cleanup(func() { cookiesFile = originalFile })

	want := map[string]string{"acf_uid": "123", "dy_did": "abc"}
	saveCookies(want)
	got := loadCookies()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loadCookies() = %#v, want %#v", got, want)
	}

	info, err := os.Stat(cookiesFile)
	if err != nil {
		t.Fatalf("stat cookies: %v", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("cookies file permissions = %o, want no group/other access", info.Mode().Perm())
	}
}

func TestLoadCookiesInvalidJSONReturnsEmpty(t *testing.T) {
	originalFile := cookiesFile
	cookiesFile = filepath.Join(t.TempDir(), "cookies.json")
	t.Cleanup(func() { cookiesFile = originalFile })
	if err := os.WriteFile(cookiesFile, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	if got := loadCookies(); len(got) != 0 {
		t.Fatalf("loadCookies() = %#v, want empty map", got)
	}
}

func TestManualCookieInputSupportsIndividualValues(t *testing.T) {
	input := strings.NewReader("\nacf_uid\n123\ndy_did\nabc\n\n")
	var output bytes.Buffer
	got := manualCookieInput(bufio.NewReader(input), &output)
	want := map[string]string{"acf_uid": "123", "dy_did": "abc"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("manualCookieInput() = %#v, want %#v", got, want)
	}
}
