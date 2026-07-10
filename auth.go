package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

var cookiesFile = "cookies.json"

// getDouyuCookies loads saved cookies or asks for them on standard input.
// The monitor normally uses Telegram recovery, but this helper preserves the
// manual cookie-entry workflow for callers that need it.
func getDouyuCookies() map[string]string {
	cookies := loadCookies()
	if len(cookies) > 0 {
		fmt.Printf("✓ Loaded cookies from %s\n", cookiesFile)
		return cookies
	}

	fmt.Println("\nNo saved cookies found.")
	fmt.Println("Please log into Douyu in your browser and copy the cookies.")
	fmt.Println("You can find cookies in DevTools > Application/Storage > Cookies")
	cookies = manualCookieInput(bufio.NewReader(os.Stdin), os.Stdout)
	if len(cookies) > 0 {
		saveCookies(cookies)
	}
	return cookies
}

// loadCookies loads cookies from cookies.json. Invalid or missing files are
// treated as an empty cookie set, matching the recovery-oriented startup flow.
func loadCookies() map[string]string {
	data, err := os.ReadFile(cookiesFile)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}
	}
	if err != nil {
		fmt.Printf("Warning: Could not load cookies file: %v\n", err)
		return map[string]string{}
	}

	var cookies map[string]string
	if err := json.Unmarshal(data, &cookies); err != nil {
		fmt.Printf("Warning: Could not load cookies file: %v\n", err)
		return map[string]string{}
	}
	if cookies == nil {
		return map[string]string{}
	}
	return cookies
}

// saveCookies stores cookies for future runs.
func saveCookies(cookies map[string]string) {
	data, err := json.MarshalIndent(cookies, "", "  ")
	if err == nil {
		err = os.WriteFile(cookiesFile, data, 0o600)
	}
	if err != nil {
		fmt.Printf("Warning: Could not save cookies file: %v\n", err)
		return
	}
	fmt.Printf("✓ Saved cookies to %s\n", cookiesFile)
}

func manualCookieInput(reader *bufio.Reader, writer io.Writer) map[string]string {
	fmt.Fprintln(writer, "\nEnter cookies (format: name1=value1; name2=value2)")
	fmt.Fprintln(writer, "Or paste individual cookie names and values:")
	fmt.Fprint(writer, "\nCookie string (or press Enter for individual input): ")

	cookieString, _ := reader.ReadString('\n')
	cookieString = strings.TrimSpace(cookieString)
	if cookieString != "" {
		return parseCookieString(cookieString)
	}

	cookies := map[string]string{}
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
		value = strings.TrimSpace(value)
		if value != "" {
			cookies[name] = value
		}
		if readErr != nil || valueErr != nil {
			break
		}
	}
	return cookies
}

// parseCookieString parses a Cookie header value into name/value pairs.
func parseCookieString(cookieString string) map[string]string {
	cleaned := normalizeCookieString(cookieString)
	cookies := map[string]string{}

	for _, pair := range strings.Split(cleaned, ";") {
		pair = strings.TrimSpace(pair)
		separator := strings.IndexByte(pair, '=')
		if pair == "" || separator < 0 {
			continue
		}

		name := strings.TrimSpace(pair[:separator])
		value := strings.TrimSpace(pair[separator+1:])
		if name != "" && value != "" {
			cookies[name] = value
		}
	}
	return cookies
}

func normalizeCookieString(cookieString string) string {
	cleaned := strings.TrimSpace(cookieString)
	if strings.HasPrefix(cleaned, "```") && strings.HasSuffix(cleaned, "```") {
		cleaned = strings.TrimSpace(strings.Trim(cleaned, "`"))
	}
	if len(cleaned) >= len("cookie:") && strings.EqualFold(cleaned[:len("cookie:")], "cookie:") {
		cleaned = strings.TrimSpace(cleaned[len("cookie:"):])
	}
	return strings.ReplaceAll(cleaned, "\n", "; ")
}
