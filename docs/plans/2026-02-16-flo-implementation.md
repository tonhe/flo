# flo Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a TUI SNMP interface monitoring tool with encrypted identity management, concurrent dashboard engines, and Base16 theme system.

**Architecture:** Single-binary hybrid monolithic app. SNMP polling runs as goroutines managed by an EngineManager. TUI layer (Bubble Tea) depends on internal packages but not vice versa. All inter-component communication via Go channels and Bubble Tea messages.

**Tech Stack:** Go 1.21+, Bubble Tea v1.2+, Lipgloss v1.0+, Bubbles v0.20+, gosnmp, BurntSushi/toml, golang.org/x/crypto (Argon2id + AES-256-GCM)

**Reference:** Design document at `docs/plans/2026-02-16-flo-design.md`. nbor codebase patterns at https://github.com/tonhe/nbor.

---

### Task 1: Project Scaffold

**Files:**
- Create: `go.mod`
- Create: `main.go`
- Create: `.gitignore`

**Step 1: Initialize Go module**

Run: `go mod init github.com/tonhe/flo`
Expected: `go.mod` created with module path

**Step 2: Create .gitignore**

```gitignore
# Binaries
flo
*.exe
*.dll
*.so
*.dylib

# Test
*.test
*.out
coverage.html

# IDE
.idea/
.vscode/
*.swp
*.swo

# OS
.DS_Store
Thumbs.db

# Session notes
claude-session-notes.md
```

**Step 3: Create main.go stub**

```go
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "flo - SNMP interface monitor")
	os.Exit(0)
}
```

**Step 4: Verify it builds**

Run: `go build -o flo . && ./flo`
Expected: prints "flo - SNMP interface monitor" to stderr

**Step 5: Commit**

```
Project scaffold with go.mod and main.go stub
```

---

### Task 2: Config Paths & App Configuration

**Files:**
- Create: `internal/config/paths.go`
- Create: `internal/config/paths_test.go`
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

**Step 1: Write paths_test.go**

```go
package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGetConfigDir(t *testing.T) {
	dir, err := GetConfigDir()
	if err != nil {
		t.Fatalf("GetConfigDir() error: %v", err)
	}
	if dir == "" {
		t.Fatal("GetConfigDir() returned empty string")
	}
	// Should end with "flo"
	if filepath.Base(dir) != "flo" {
		t.Errorf("expected dir to end with 'flo', got %q", filepath.Base(dir))
	}
}

func TestGetConfigDirXDG(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("XDG test not applicable on Windows")
	}
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	dir, err := GetConfigDir()
	if err != nil {
		t.Fatalf("GetConfigDir() error: %v", err)
	}
	expected := filepath.Join(tmp, "flo")
	if dir != expected {
		t.Errorf("expected %q, got %q", expected, dir)
	}
}

func TestGetDataDir(t *testing.T) {
	dir, err := GetDataDir()
	if err != nil {
		t.Fatalf("GetDataDir() error: %v", err)
	}
	if dir == "" {
		t.Fatal("GetDataDir() returned empty string")
	}
	if filepath.Base(dir) != "flo" {
		t.Errorf("expected dir to end with 'flo', got %q", filepath.Base(dir))
	}
}

func TestDashboardsDir(t *testing.T) {
	dir, err := GetDashboardsDir()
	if err != nil {
		t.Fatalf("GetDashboardsDir() error: %v", err)
	}
	if filepath.Base(dir) != "dashboards" {
		t.Errorf("expected dir to end with 'dashboards', got %q", filepath.Base(dir))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -v -run TestGetConfigDir`
Expected: FAIL - package doesn't exist yet

**Step 3: Implement paths.go**

```go
package config

import (
	"os"
	"path/filepath"
	"runtime"
)

const appName = "flo"

// GetConfigDir returns the platform-specific config directory.
// Unix: $XDG_CONFIG_HOME/flo or ~/.config/flo
// Windows: %APPDATA%\flo
func GetConfigDir() (string, error) {
	var base string
	switch runtime.GOOS {
	case "windows":
		base = os.Getenv("APPDATA")
		if base == "" {
			base = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming")
		}
	default:
		base = os.Getenv("XDG_CONFIG_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".config")
		}
	}
	return filepath.Join(base, appName), nil
}

// GetDataDir returns the platform-specific data directory.
// Unix: $XDG_DATA_HOME/flo or ~/.local/share/flo
// Windows: %LOCALAPPDATA%\flo
func GetDataDir() (string, error) {
	var base string
	switch runtime.GOOS {
	case "windows":
		base = os.Getenv("LOCALAPPDATA")
		if base == "" {
			base = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}
	default:
		base = os.Getenv("XDG_DATA_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".local", "share")
		}
	}
	return filepath.Join(base, appName), nil
}

// GetDashboardsDir returns the directory for dashboard config files.
func GetDashboardsDir() (string, error) {
	cfgDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfgDir, "dashboards"), nil
}

// GetIdentityStorePath returns the path to the encrypted identity store.
func GetIdentityStorePath() (string, error) {
	cfgDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfgDir, "identities.enc"), nil
}

// EnsureDirs creates all required directories if they don't exist.
func EnsureDirs() error {
	dirs := []func() (string, error){GetConfigDir, GetDataDir, GetDashboardsDir}
	for _, fn := range dirs {
		dir, err := fn()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}
	return nil
}
```

**Step 4: Run tests**

Run: `go test ./internal/config/ -v -run TestGet`
Expected: All PASS

**Step 5: Write config_test.go**

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Theme != "solarized-dark" {
		t.Errorf("expected default theme 'solarized-dark', got %q", cfg.Theme)
	}
	if cfg.PollInterval != 10*time.Second {
		t.Errorf("expected poll interval 10s, got %v", cfg.PollInterval)
	}
	if cfg.MaxHistory != 360 {
		t.Errorf("expected max history 360, got %d", cfg.MaxHistory)
	}
}

func TestConfigSaveLoad(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.toml")

	cfg := DefaultConfig()
	cfg.Theme = "dracula"
	cfg.DefaultIdentity = "test-id"

	if err := SaveConfig(cfg, path); err != nil {
		t.Fatalf("SaveConfig() error: %v", err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	if loaded.Theme != "dracula" {
		t.Errorf("expected theme 'dracula', got %q", loaded.Theme)
	}
	if loaded.DefaultIdentity != "test-id" {
		t.Errorf("expected identity 'test-id', got %q", loaded.DefaultIdentity)
	}
}

func TestConfigLoadMissing(t *testing.T) {
	cfg, err := LoadConfig("/nonexistent/path/config.toml")
	if err != nil {
		t.Fatalf("LoadConfig() should return defaults for missing file, got error: %v", err)
	}
	if cfg.Theme != "solarized-dark" {
		t.Errorf("expected default theme, got %q", cfg.Theme)
	}
}
```

**Step 6: Run test to verify it fails**

Run: `go test ./internal/config/ -v -run TestDefault`
Expected: FAIL

**Step 7: Implement config.go**

```go
package config

import (
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Theme           string        `toml:"theme"`
	DefaultIdentity string        `toml:"default_identity"`
	PollInterval    time.Duration `toml:"-"`
	PollIntervalStr string        `toml:"poll_interval"`
	MaxHistory      int           `toml:"max_history"`
}

func DefaultConfig() *Config {
	return &Config{
		Theme:           "solarized-dark",
		DefaultIdentity: "",
		PollInterval:    10 * time.Second,
		PollIntervalStr: "10s",
		MaxHistory:      360,
	}
}

func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	if cfg.PollIntervalStr != "" {
		d, err := time.ParseDuration(cfg.PollIntervalStr)
		if err == nil {
			cfg.PollInterval = d
		}
	}
	return cfg, nil
}

func SaveConfig(cfg *Config, path string) error {
	cfg.PollIntervalStr = cfg.PollInterval.String()
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(cfg)
}
```

**Step 8: Install dependency and run tests**

Run: `go get github.com/BurntSushi/toml@v1.6.0 && go test ./internal/config/ -v`
Expected: All PASS

**Step 9: Commit**

```
Add config paths and app configuration with TOML persistence
```

---

### Task 3: Theme System

Port nbor's Base16 theme system.

**Files:**
- Create: `tui/styles/theme.go`
- Create: `tui/styles/theme_test.go`
- Create: `tui/styles/themes_data.go`
- Create: `tui/styles/styles.go`

**Step 1: Write theme_test.go**

```go
package styles

import (
	"testing"
)

func TestGetThemeByName(t *testing.T) {
	theme := GetThemeByName("solarized-dark")
	if theme == nil {
		t.Fatal("GetThemeByName('solarized-dark') returned nil")
	}
	if theme.Name != "Solarized Dark" {
		t.Errorf("expected name 'Solarized Dark', got %q", theme.Name)
	}
}

func TestGetThemeByNameMissing(t *testing.T) {
	theme := GetThemeByName("nonexistent")
	if theme != nil {
		t.Error("expected nil for nonexistent theme")
	}
}

func TestListThemes(t *testing.T) {
	themes := ListThemes()
	if len(themes) < 20 {
		t.Errorf("expected at least 20 themes, got %d", len(themes))
	}
}

func TestThemeCount(t *testing.T) {
	count := GetThemeCount()
	if count < 20 {
		t.Errorf("expected at least 20 themes, got %d", count)
	}
}

func TestGetThemeByIndex(t *testing.T) {
	theme := GetThemeByIndex(0)
	if theme == nil {
		t.Fatal("GetThemeByIndex(0) returned nil")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./tui/styles/ -v`
Expected: FAIL

**Step 3: Implement theme.go**

```go
package styles

import (
	"sort"

	"github.com/charmbracelet/lipgloss"
)

// Theme represents a Base16 color scheme.
type Theme struct {
	Name   string
	Base00 lipgloss.Color // Background
	Base01 lipgloss.Color // Lighter background
	Base02 lipgloss.Color // Selection
	Base03 lipgloss.Color // Comments / dim
	Base04 lipgloss.Color // Light foreground
	Base05 lipgloss.Color // Foreground
	Base06 lipgloss.Color // Light foreground
	Base07 lipgloss.Color // Light background
	Base08 lipgloss.Color // Red
	Base09 lipgloss.Color // Orange
	Base0A lipgloss.Color // Yellow
	Base0B lipgloss.Color // Green
	Base0C lipgloss.Color // Cyan
	Base0D lipgloss.Color // Blue
	Base0E lipgloss.Color // Magenta
	Base0F lipgloss.Color // Brown
}

var (
	DefaultTheme  Theme
	sortedSlugs   []string
)

func init() {
	sortedSlugs = make([]string, 0, len(Themes))
	for slug := range Themes {
		sortedSlugs = append(sortedSlugs, slug)
	}
	sort.Strings(sortedSlugs)
	DefaultTheme = Themes["solarized-dark"]
}

// SetTheme updates the default theme.
func SetTheme(theme Theme) {
	DefaultTheme = theme
}

// GetThemeByName returns a theme by its slug, or nil if not found.
func GetThemeByName(name string) *Theme {
	t, ok := Themes[name]
	if !ok {
		return nil
	}
	return &t
}

// ListThemes returns sorted theme slugs.
func ListThemes() []string {
	return sortedSlugs
}

// GetThemeCount returns the total number of available themes.
func GetThemeCount() int {
	return len(Themes)
}

// GetThemeByIndex returns a theme at the given sorted index.
func GetThemeByIndex(idx int) *Theme {
	if idx < 0 || idx >= len(sortedSlugs) {
		return nil
	}
	t := Themes[sortedSlugs[idx]]
	return &t
}

// GetThemeIndex returns the sorted index of a theme slug, or -1.
func GetThemeIndex(slug string) int {
	for i, s := range sortedSlugs {
		if s == slug {
			return i
		}
	}
	return -1
}
```

**Step 4: Implement themes_data.go**

Create all 21 themes matching nbor's set. Each theme is a Base16 color scheme defined with hex colors. Include at minimum: solarized-dark, solarized-light, gruvbox-dark, gruvbox-light, dracula, nord, one-dark, monokai, tokyo-night, catppuccin-mocha, catppuccin-latte, everforest, kanagawa, ayu-dark, horizon, zenburn, palenight, github-dark, tomorrow-night, rose-pine, and a default "flo" theme.

```go
package styles

import "github.com/charmbracelet/lipgloss"

// Themes is the registry of all built-in themes.
var Themes = map[string]Theme{
	"solarized-dark": {
		Name:   "Solarized Dark",
		Base00: lipgloss.Color("#002b36"),
		Base01: lipgloss.Color("#073642"),
		Base02: lipgloss.Color("#586e75"),
		Base03: lipgloss.Color("#657b83"),
		Base04: lipgloss.Color("#839496"),
		Base05: lipgloss.Color("#93a1a1"),
		Base06: lipgloss.Color("#eee8d5"),
		Base07: lipgloss.Color("#fdf6e3"),
		Base08: lipgloss.Color("#dc322f"),
		Base09: lipgloss.Color("#cb4b16"),
		Base0A: lipgloss.Color("#b58900"),
		Base0B: lipgloss.Color("#859900"),
		Base0C: lipgloss.Color("#2aa198"),
		Base0D: lipgloss.Color("#268bd2"),
		Base0E: lipgloss.Color("#6c71c4"),
		Base0F: lipgloss.Color("#d33682"),
	},
	// ... all other themes (port from nbor's themes_data.go)
}
```

Note: Copy all 21 theme definitions from nbor's `tui/themes_data.go`. Each follows the same struct pattern with 16 Base16 color values.

**Step 5: Implement styles.go**

```go
package styles

import "github.com/charmbracelet/lipgloss"

// Styles holds all themed lipgloss styles for the application.
type Styles struct {
	// Layout
	AppContainer lipgloss.Style

	// Header / Footer
	Header       lipgloss.Style
	HeaderTitle  lipgloss.Style
	HeaderStatus lipgloss.Style
	Footer       lipgloss.Style
	FooterKey    lipgloss.Style
	FooterDesc   lipgloss.Style

	// Table
	TableHeader  lipgloss.Style
	TableRow     lipgloss.Style
	TableRowSel  lipgloss.Style
	TableCellDim lipgloss.Style

	// Status colors
	StatusUp   lipgloss.Style
	StatusDown lipgloss.Style
	StatusWarn lipgloss.Style

	// Utilization thresholds
	UtilLow  lipgloss.Style // < 50%
	UtilMid  lipgloss.Style // 50-80%
	UtilHigh lipgloss.Style // > 80%

	// Sparkline
	SparklineStyle lipgloss.Style

	// Groups
	GroupHeader lipgloss.Style

	// Modal / overlay
	ModalBorder lipgloss.Style
	ModalTitle  lipgloss.Style

	// Form
	FormLabel       lipgloss.Style
	FormInput       lipgloss.Style
	FormInputActive lipgloss.Style
	FormCursor      lipgloss.Style

	// Identity table
	IdentityName    lipgloss.Style
	IdentityVersion lipgloss.Style
}

// NewStyles creates a new Styles instance from a theme.
func NewStyles(theme Theme) *Styles {
	return &Styles{
		AppContainer: lipgloss.NewStyle().
			Foreground(theme.Base05).
			Background(theme.Base00),

		Header: lipgloss.NewStyle().
			Foreground(theme.Base05).
			Background(theme.Base01).
			Bold(true).
			Padding(0, 1),
		HeaderTitle: lipgloss.NewStyle().
			Foreground(theme.Base0D).
			Bold(true),
		HeaderStatus: lipgloss.NewStyle().
			Foreground(theme.Base0B),

		Footer: lipgloss.NewStyle().
			Foreground(theme.Base04).
			Background(theme.Base01).
			Padding(0, 1),
		FooterKey: lipgloss.NewStyle().
			Foreground(theme.Base0D).
			Bold(true),
		FooterDesc: lipgloss.NewStyle().
			Foreground(theme.Base04),

		TableHeader: lipgloss.NewStyle().
			Foreground(theme.Base0D).
			Bold(true),
		TableRow: lipgloss.NewStyle().
			Foreground(theme.Base05),
		TableRowSel: lipgloss.NewStyle().
			Foreground(theme.Base05).
			Background(theme.Base02),
		TableCellDim: lipgloss.NewStyle().
			Foreground(theme.Base03),

		StatusUp: lipgloss.NewStyle().
			Foreground(theme.Base0B),
		StatusDown: lipgloss.NewStyle().
			Foreground(theme.Base08),
		StatusWarn: lipgloss.NewStyle().
			Foreground(theme.Base0A),

		UtilLow: lipgloss.NewStyle().
			Foreground(theme.Base0B),
		UtilMid: lipgloss.NewStyle().
			Foreground(theme.Base0A),
		UtilHigh: lipgloss.NewStyle().
			Foreground(theme.Base08),

		SparklineStyle: lipgloss.NewStyle().
			Foreground(theme.Base0C),

		GroupHeader: lipgloss.NewStyle().
			Foreground(theme.Base0E).
			Bold(true),

		ModalBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Base0D).
			Padding(1, 2),
		ModalTitle: lipgloss.NewStyle().
			Foreground(theme.Base0D).
			Bold(true),

		FormLabel: lipgloss.NewStyle().
			Foreground(theme.Base04),
		FormInput: lipgloss.NewStyle().
			Foreground(theme.Base05),
		FormInputActive: lipgloss.NewStyle().
			Foreground(theme.Base06).
			Background(theme.Base02),
		FormCursor: lipgloss.NewStyle().
			Foreground(theme.Base0B),

		IdentityName: lipgloss.NewStyle().
			Foreground(theme.Base0D),
		IdentityVersion: lipgloss.NewStyle().
			Foreground(theme.Base0C),
	}
}
```

**Step 6: Install lipgloss dependency and run tests**

Run: `go get github.com/charmbracelet/lipgloss@v1.0.0 && go test ./tui/styles/ -v`
Expected: All PASS

**Step 7: Commit**

```
Add Base16 theme system with 21 themes and lipgloss styles
```

---

### Task 4: Identity Types & Crypto

**Files:**
- Create: `internal/identity/identity.go`
- Create: `internal/identity/crypto.go`
- Create: `internal/identity/crypto_test.go`

**Step 1: Write identity.go (types only, no tests needed for pure data)**

```go
package identity

// Identity represents an SNMP credential profile.
type Identity struct {
	Name      string `json:"name"`
	Version   string `json:"version"`    // "1", "2c", "3"
	Community string `json:"community"`  // v1/v2c
	Username  string `json:"username"`   // v3
	AuthProto string `json:"auth_proto"` // "MD5", "SHA", "SHA256", "SHA512"
	AuthPass  string `json:"auth_pass"`
	PrivProto string `json:"priv_proto"` // "DES", "AES128", "AES192", "AES256"
	PrivPass  string `json:"priv_pass"`
}

// Summary returns a safe representation without secrets.
type Summary struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Username  string `json:"username,omitempty"`
	AuthProto string `json:"auth_proto,omitempty"`
	PrivProto string `json:"priv_proto,omitempty"`
}

// Summarize returns a Summary without sensitive fields.
func (id *Identity) Summarize() Summary {
	return Summary{
		Name:      id.Name,
		Version:   id.Version,
		Username:  id.Username,
		AuthProto: id.AuthProto,
		PrivProto: id.PrivProto,
	}
}

// Provider is the interface for identity storage backends.
type Provider interface {
	List() ([]Summary, error)
	Get(name string) (*Identity, error)
	Add(id Identity) error
	Update(name string, id Identity) error
	Remove(name string) error
}
```

**Step 2: Write crypto_test.go**

```go
package identity

import (
	"bytes"
	"testing"
)

func TestDeriveKey(t *testing.T) {
	salt := make([]byte, 16)
	key := DeriveKey([]byte("test-password"), salt)
	if len(key) != 32 {
		t.Errorf("expected 32 byte key, got %d", len(key))
	}
}

func TestDeriveKeyDeterministic(t *testing.T) {
	salt := []byte("fixed-salt-value")
	key1 := DeriveKey([]byte("password"), salt)
	key2 := DeriveKey([]byte("password"), salt)
	if !bytes.Equal(key1, key2) {
		t.Error("same password+salt should produce same key")
	}
}

func TestDeriveKeyDifferentPasswords(t *testing.T) {
	salt := []byte("fixed-salt-value")
	key1 := DeriveKey([]byte("password1"), salt)
	key2 := DeriveKey([]byte("password2"), salt)
	if bytes.Equal(key1, key2) {
		t.Error("different passwords should produce different keys")
	}
}

func TestEncryptDecrypt(t *testing.T) {
	key := make([]byte, 32)
	plaintext := []byte("secret data for testing")

	ciphertext, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error: %v", err)
	}
	if bytes.Equal(ciphertext, plaintext) {
		t.Error("ciphertext should differ from plaintext")
	}

	decrypted, err := Decrypt(key, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt() error: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("decrypted data doesn't match: got %q, want %q", decrypted, plaintext)
	}
}

func TestDecryptWrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	key2[0] = 1 // different key

	ciphertext, err := Encrypt(key1, []byte("secret"))
	if err != nil {
		t.Fatalf("Encrypt() error: %v", err)
	}

	_, err = Decrypt(key2, ciphertext)
	if err == nil {
		t.Error("Decrypt with wrong key should fail")
	}
}

func TestGenerateSalt(t *testing.T) {
	salt, err := GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt() error: %v", err)
	}
	if len(salt) != 16 {
		t.Errorf("expected 16 byte salt, got %d", len(salt))
	}
}
```

**Step 3: Run test to verify it fails**

Run: `go test ./internal/identity/ -v -run TestDeriveKey`
Expected: FAIL

**Step 4: Implement crypto.go**

```go
package identity

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"

	"golang.org/x/crypto/argon2"
)

const (
	saltLen    = 16
	keyLen     = 32 // AES-256
	argonTime  = 1
	argonMem   = 64 * 1024 // 64 MB
	argonThreads = 4
)

// DeriveKey derives a 32-byte encryption key from a password and salt using Argon2id.
func DeriveKey(password, salt []byte) []byte {
	return argon2.IDKey(password, salt, argonTime, argonMem, argonThreads, keyLen)
}

// GenerateSalt generates a random 16-byte salt.
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	return salt, nil
}

// Encrypt encrypts plaintext with AES-256-GCM using the given key.
// Returns nonce + ciphertext.
func Encrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt decrypts AES-256-GCM ciphertext (nonce prepended) with the given key.
func Decrypt(key, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
```

**Step 5: Install dependency and run tests**

Run: `go get golang.org/x/crypto && go test ./internal/identity/ -v`
Expected: All PASS

**Step 6: Commit**

```
Add identity types and AES-256-GCM crypto with Argon2id key derivation
```

---

### Task 5: Identity Store (Encrypted File Backend)

**Files:**
- Create: `internal/identity/store.go`
- Create: `internal/identity/store_test.go`

**Step 1: Write store_test.go**

```go
package identity

import (
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *FileStore {
	t.Helper()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "identities.enc")
	store, err := NewFileStore(path, []byte("test-master-password"))
	if err != nil {
		t.Fatalf("NewFileStore() error: %v", err)
	}
	return store
}

func TestStoreAddAndGet(t *testing.T) {
	store := newTestStore(t)
	id := Identity{
		Name:    "test-v2c",
		Version: "2c",
		Community: "public",
	}
	if err := store.Add(id); err != nil {
		t.Fatalf("Add() error: %v", err)
	}
	got, err := store.Get("test-v2c")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got.Community != "public" {
		t.Errorf("expected community 'public', got %q", got.Community)
	}
}

func TestStoreList(t *testing.T) {
	store := newTestStore(t)
	store.Add(Identity{Name: "a", Version: "2c", Community: "x"})
	store.Add(Identity{Name: "b", Version: "3", Username: "user"})

	summaries, err := store.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(summaries) != 2 {
		t.Errorf("expected 2 summaries, got %d", len(summaries))
	}
}

func TestStoreRemove(t *testing.T) {
	store := newTestStore(t)
	store.Add(Identity{Name: "x", Version: "2c", Community: "test"})
	if err := store.Remove("x"); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}
	_, err := store.Get("x")
	if err == nil {
		t.Error("expected error after removing identity")
	}
}

func TestStoreUpdate(t *testing.T) {
	store := newTestStore(t)
	store.Add(Identity{Name: "x", Version: "2c", Community: "old"})
	err := store.Update("x", Identity{Name: "x", Version: "2c", Community: "new"})
	if err != nil {
		t.Fatalf("Update() error: %v", err)
	}
	got, _ := store.Get("x")
	if got.Community != "new" {
		t.Errorf("expected 'new', got %q", got.Community)
	}
}

func TestStorePersistence(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "identities.enc")
	password := []byte("test-password")

	// Create and add
	store1, _ := NewFileStore(path, password)
	store1.Add(Identity{Name: "persist", Version: "2c", Community: "test"})

	// Reopen with same password
	store2, err := NewFileStore(path, password)
	if err != nil {
		t.Fatalf("reopen error: %v", err)
	}
	got, err := store2.Get("persist")
	if err != nil {
		t.Fatalf("Get() after reopen error: %v", err)
	}
	if got.Community != "test" {
		t.Errorf("expected 'test', got %q", got.Community)
	}
}

func TestStoreWrongPassword(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "identities.enc")

	store1, _ := NewFileStore(path, []byte("correct"))
	store1.Add(Identity{Name: "x", Version: "2c", Community: "test"})

	_, err := NewFileStore(path, []byte("wrong"))
	if err == nil {
		t.Error("expected error with wrong password")
	}
}

func TestStoreDuplicateAdd(t *testing.T) {
	store := newTestStore(t)
	store.Add(Identity{Name: "dup", Version: "2c", Community: "a"})
	err := store.Add(Identity{Name: "dup", Version: "2c", Community: "b"})
	if err == nil {
		t.Error("expected error adding duplicate identity name")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/identity/ -v -run TestStore`
Expected: FAIL

**Step 3: Implement store.go**

```go
package identity

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
)

var (
	ErrNotFound  = errors.New("identity not found")
	ErrDuplicate = errors.New("identity already exists")
	ErrDecrypt   = errors.New("failed to decrypt identity store (wrong password?)")
)

// storeFile is the on-disk format: salt + encrypted JSON blob.
type storeFile struct {
	Salt []byte `json:"salt"`
	Data []byte `json:"data"`
}

// FileStore implements Provider with an AES-256-GCM encrypted JSON file.
type FileStore struct {
	mu         sync.RWMutex
	path       string
	key        []byte
	salt       []byte
	identities map[string]Identity
}

// NewFileStore opens or creates an encrypted identity store.
func NewFileStore(path string, password []byte) (*FileStore, error) {
	s := &FileStore{
		path:       path,
		identities: make(map[string]Identity),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// New store
			salt, err := GenerateSalt()
			if err != nil {
				return nil, err
			}
			s.salt = salt
			s.key = DeriveKey(password, salt)
			return s, s.save()
		}
		return nil, err
	}

	// Existing store - parse envelope
	var sf storeFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return nil, fmt.Errorf("corrupt identity store: %w", err)
	}

	s.salt = sf.Salt
	s.key = DeriveKey(password, sf.Salt)

	// Decrypt
	plaintext, err := Decrypt(s.key, sf.Data)
	if err != nil {
		return nil, ErrDecrypt
	}

	if err := json.Unmarshal(plaintext, &s.identities); err != nil {
		return nil, fmt.Errorf("corrupt identity data: %w", err)
	}
	return s, nil
}

func (s *FileStore) save() error {
	plaintext, err := json.Marshal(s.identities)
	if err != nil {
		return err
	}
	encrypted, err := Encrypt(s.key, plaintext)
	if err != nil {
		return err
	}
	sf := storeFile{Salt: s.salt, Data: encrypted}
	data, err := json.Marshal(sf)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0600)
}

func (s *FileStore) List() ([]Summary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	summaries := make([]Summary, 0, len(s.identities))
	for _, id := range s.identities {
		summaries = append(summaries, id.Summarize())
	}
	return summaries, nil
}

func (s *FileStore) Get(name string) (*Identity, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.identities[name]
	if !ok {
		return nil, ErrNotFound
	}
	return &id, nil
}

func (s *FileStore) Add(id Identity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.identities[id.Name]; exists {
		return ErrDuplicate
	}
	s.identities[id.Name] = id
	return s.save()
}

func (s *FileStore) Update(name string, id Identity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.identities[name]; !exists {
		return ErrNotFound
	}
	if name != id.Name {
		delete(s.identities, name)
	}
	s.identities[id.Name] = id
	return s.save()
}

func (s *FileStore) Remove(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.identities[name]; !exists {
		return ErrNotFound
	}
	delete(s.identities, name)
	return s.save()
}
```

**Step 4: Run tests**

Run: `go test ./internal/identity/ -v`
Expected: All PASS

**Step 5: Commit**

```
Add encrypted identity store with file-based persistence
```

---

### Task 6: SNMP Wrapper & Ring Buffer

**Files:**
- Create: `internal/engine/snmp.go`
- Create: `internal/engine/snmp_test.go`
- Create: `internal/engine/ringbuffer.go`
- Create: `internal/engine/ringbuffer_test.go`
- Create: `internal/engine/rate.go`
- Create: `internal/engine/rate_test.go`

**Step 1: Write ringbuffer_test.go**

```go
package engine

import (
	"testing"
	"time"
)

func TestRingBufferAdd(t *testing.T) {
	rb := NewRingBuffer[RateSample](5)
	for i := 0; i < 3; i++ {
		rb.Add(RateSample{Timestamp: time.Now(), InRate: float64(i)})
	}
	if rb.Len() != 3 {
		t.Errorf("expected len 3, got %d", rb.Len())
	}
}

func TestRingBufferWrap(t *testing.T) {
	rb := NewRingBuffer[RateSample](3)
	for i := 0; i < 5; i++ {
		rb.Add(RateSample{InRate: float64(i)})
	}
	if rb.Len() != 3 {
		t.Errorf("expected len 3, got %d", rb.Len())
	}
	items := rb.All()
	if items[0].InRate != 2 {
		t.Errorf("expected oldest item InRate=2, got %f", items[0].InRate)
	}
	if items[2].InRate != 4 {
		t.Errorf("expected newest item InRate=4, got %f", items[2].InRate)
	}
}

func TestRingBufferEmpty(t *testing.T) {
	rb := NewRingBuffer[RateSample](10)
	if rb.Len() != 0 {
		t.Error("new ring buffer should be empty")
	}
	items := rb.All()
	if len(items) != 0 {
		t.Error("All() on empty buffer should return empty slice")
	}
}

func TestRingBufferLast(t *testing.T) {
	rb := NewRingBuffer[RateSample](5)
	rb.Add(RateSample{InRate: 1})
	rb.Add(RateSample{InRate: 2})
	rb.Add(RateSample{InRate: 3})
	last, ok := rb.Last()
	if !ok {
		t.Fatal("Last() should return true for non-empty buffer")
	}
	if last.InRate != 3 {
		t.Errorf("expected InRate=3, got %f", last.InRate)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/engine/ -v -run TestRing`
Expected: FAIL

**Step 3: Implement ringbuffer.go**

```go
package engine

import "sync"

// RingBuffer is a fixed-size circular buffer.
type RingBuffer[T any] struct {
	mu    sync.RWMutex
	items []T
	head  int
	count int
	cap   int
}

func NewRingBuffer[T any](capacity int) *RingBuffer[T] {
	return &RingBuffer[T]{
		items: make([]T, capacity),
		cap:   capacity,
	}
}

func (r *RingBuffer[T]) Add(item T) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[r.head] = item
	r.head = (r.head + 1) % r.cap
	if r.count < r.cap {
		r.count++
	}
}

func (r *RingBuffer[T]) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.count
}

// All returns items from oldest to newest.
func (r *RingBuffer[T]) All() []T {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]T, r.count)
	start := 0
	if r.count == r.cap {
		start = r.head
	}
	for i := 0; i < r.count; i++ {
		result[i] = r.items[(start+i)%r.cap]
	}
	return result
}

// Last returns the most recently added item.
func (r *RingBuffer[T]) Last() (T, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var zero T
	if r.count == 0 {
		return zero, false
	}
	idx := (r.head - 1 + r.cap) % r.cap
	return r.items[idx], true
}
```

**Step 4: Run ring buffer tests**

Run: `go test ./internal/engine/ -v -run TestRing`
Expected: All PASS

**Step 5: Write rate_test.go**

```go
package engine

import (
	"testing"
	"time"
)

func TestCalculateRate(t *testing.T) {
	prev := CounterSample{
		InOctets:  1000,
		OutOctets: 500,
		Timestamp: time.Now().Add(-10 * time.Second),
	}
	curr := CounterSample{
		InOctets:  2000,
		OutOctets: 1500,
		Timestamp: time.Now(),
	}
	rate, err := CalculateRate(prev, curr)
	if err != nil {
		t.Fatalf("CalculateRate() error: %v", err)
	}
	// (1000 bytes * 8 bits) / 10 seconds = 800 bps
	if rate.InRate < 790 || rate.InRate > 810 {
		t.Errorf("expected InRate ~800, got %f", rate.InRate)
	}
	if rate.OutRate < 790 || rate.OutRate > 810 {
		t.Errorf("expected OutRate ~800, got %f", rate.OutRate)
	}
}

func TestCalculateRateCounterWrap(t *testing.T) {
	prev := CounterSample{
		InOctets:  100,
		OutOctets: 50,
		Timestamp: time.Now().Add(-10 * time.Second),
	}
	curr := CounterSample{
		InOctets:  50, // wrapped
		OutOctets: 50,
		Timestamp: time.Now(),
	}
	_, err := CalculateRate(prev, curr)
	if err != ErrCounterWrap {
		t.Errorf("expected ErrCounterWrap, got %v", err)
	}
}

func TestCalculateUtilization(t *testing.T) {
	util := CalculateUtilization(500_000_000, 300_000_000, 1000)
	// max(500Mbps, 300Mbps) / 1000Mbps = 50%
	if util < 49 || util > 51 {
		t.Errorf("expected ~50%%, got %f", util)
	}
}
```

**Step 6: Implement rate.go**

```go
package engine

import (
	"errors"
	"time"
)

var ErrCounterWrap = errors.New("counter wrap detected")

// CounterSample holds raw SNMP counter values at a point in time.
type CounterSample struct {
	InOctets  uint64
	OutOctets uint64
	Timestamp time.Time
}

// RateSample holds calculated rates in bits per second.
type RateSample struct {
	Timestamp time.Time
	InRate    float64
	OutRate   float64
}

// CalculateRate computes bits-per-second rates between two counter samples.
func CalculateRate(prev, curr CounterSample) (RateSample, error) {
	elapsed := curr.Timestamp.Sub(prev.Timestamp).Seconds()
	if elapsed <= 0 {
		return RateSample{}, errors.New("zero or negative elapsed time")
	}

	if curr.InOctets < prev.InOctets || curr.OutOctets < prev.OutOctets {
		return RateSample{}, ErrCounterWrap
	}

	deltaIn := curr.InOctets - prev.InOctets
	deltaOut := curr.OutOctets - prev.OutOctets

	return RateSample{
		Timestamp: curr.Timestamp,
		InRate:    float64(deltaIn) * 8 / elapsed,
		OutRate:   float64(deltaOut) * 8 / elapsed,
	}, nil
}

// CalculateUtilization returns utilization percentage.
// inRate and outRate in bps, speed in Mbps.
func CalculateUtilization(inRate, outRate float64, speedMbps uint64) float64 {
	if speedMbps == 0 {
		return 0
	}
	maxRate := inRate
	if outRate > maxRate {
		maxRate = outRate
	}
	return maxRate / (float64(speedMbps) * 1_000_000) * 100
}
```

**Step 7: Run all engine tests**

Run: `go test ./internal/engine/ -v`
Expected: All PASS

**Step 8: Implement snmp.go (gosnmp wrapper)**

```go
package engine

import (
	"fmt"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/tonhe/flo/internal/identity"
)

// Standard IF-MIB OIDs
const (
	OIDifName       = "1.3.6.1.2.1.31.1.1.1.1"
	OIDifDescr      = "1.3.6.1.2.1.2.2.1.2"
	OIDifAlias      = "1.3.6.1.2.1.31.1.1.1.18"
	OIDifHCInOctets = "1.3.6.1.2.1.31.1.1.1.6"
	OIDifHCOutOctets = "1.3.6.1.2.1.31.1.1.1.10"
	OIDifHighSpeed  = "1.3.6.1.2.1.31.1.1.1.15"
	OIDifOperStatus = "1.3.6.1.2.1.2.2.1.8"
)

// NewSNMPClient creates a gosnmp.GoSNMP from an Identity.
func NewSNMPClient(host string, port int, id *identity.Identity, timeout time.Duration) (*gosnmp.GoSNMP, error) {
	if port == 0 {
		port = 161
	}
	client := &gosnmp.GoSNMP{
		Target:  host,
		Port:    uint16(port),
		Timeout: timeout,
		Retries: 2,
	}

	switch id.Version {
	case "1":
		client.Version = gosnmp.Version1
		client.Community = id.Community
	case "2c":
		client.Version = gosnmp.Version2c
		client.Community = id.Community
	case "3":
		client.Version = gosnmp.Version3
		client.SecurityModel = gosnmp.UserSecurityModel
		client.MsgFlags = snmpv3MsgFlags(id)
		client.SecurityParameters = &gosnmp.UsmSecurityParameters{
			UserName:                 id.Username,
			AuthenticationProtocol:   snmpv3AuthProto(id.AuthProto),
			AuthenticationPassphrase: id.AuthPass,
			PrivacyProtocol:          snmpv3PrivProto(id.PrivProto),
			PrivacyPassphrase:        id.PrivPass,
		}
	default:
		return nil, fmt.Errorf("unsupported SNMP version: %s", id.Version)
	}
	return client, nil
}

func snmpv3MsgFlags(id *identity.Identity) gosnmp.SnmpV3MsgFlags {
	if id.PrivProto != "" && id.PrivPass != "" {
		return gosnmp.AuthPriv
	}
	if id.AuthProto != "" && id.AuthPass != "" {
		return gosnmp.AuthNoPriv
	}
	return gosnmp.NoAuthNoPriv
}

func snmpv3AuthProto(proto string) gosnmp.SnmpV3AuthProtocol {
	switch proto {
	case "MD5":
		return gosnmp.MD5
	case "SHA":
		return gosnmp.SHA
	case "SHA256":
		return gosnmp.SHA256
	case "SHA512":
		return gosnmp.SHA512
	default:
		return gosnmp.NoAuth
	}
}

func snmpv3PrivProto(proto string) gosnmp.SnmpV3PrivProtocol {
	switch proto {
	case "DES":
		return gosnmp.DES
	case "AES", "AES128":
		return gosnmp.AES
	case "AES192":
		return gosnmp.AES192
	case "AES256":
		return gosnmp.AES256
	default:
		return gosnmp.NoPriv
	}
}
```

**Step 9: Install gosnmp and run build**

Run: `go get github.com/gosnmp/gosnmp && go build ./...`
Expected: Build succeeds

**Step 10: Commit**

```
Add SNMP wrapper, ring buffer, and rate calculator
```

---

### Task 7: Dashboard Types & TOML Loader

**Files:**
- Create: `internal/dashboard/dashboard.go`
- Create: `internal/dashboard/loader.go`
- Create: `internal/dashboard/loader_test.go`

**Step 1: Write dashboard.go (types)**

```go
package dashboard

import "time"

type Dashboard struct {
	Name            string   `toml:"name"`
	DefaultIdentity string   `toml:"default_identity"`
	IntervalStr     string   `toml:"interval"`
	Interval        time.Duration `toml:"-"`
	MaxHistory      int      `toml:"max_history"`
	Groups          []Group  `toml:"groups"`
}

type Group struct {
	Name    string   `toml:"name"`
	Targets []Target `toml:"targets"`
}

type Target struct {
	Host       string   `toml:"host"`
	Label      string   `toml:"label"`
	Identity   string   `toml:"identity"`
	Port       int      `toml:"port"`
	Interfaces []string `toml:"interfaces"`
}
```

**Step 2: Write loader_test.go**

```go
package dashboard

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const testDashboardTOML = `
name = "Test Dashboard"
default_identity = "test-v2c"
interval = "10s"
max_history = 360

[[groups]]
name = "Core"

[[groups.targets]]
host = "10.0.1.1"
label = "rtr-1"
identity = "test-v2c"
interfaces = ["Gi0/0", "Gi0/1"]

[[groups]]
name = "Branch"

[[groups.targets]]
host = "10.1.1.1"
label = "branch-1"
identity = "branch-ro"
port = 1161
interfaces = ["Eth1"]
`

func TestLoadDashboard(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.toml")
	os.WriteFile(path, []byte(testDashboardTOML), 0644)

	dash, err := LoadDashboard(path)
	if err != nil {
		t.Fatalf("LoadDashboard() error: %v", err)
	}
	if dash.Name != "Test Dashboard" {
		t.Errorf("expected name 'Test Dashboard', got %q", dash.Name)
	}
	if dash.Interval != 10*time.Second {
		t.Errorf("expected interval 10s, got %v", dash.Interval)
	}
	if len(dash.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(dash.Groups))
	}
	if len(dash.Groups[0].Targets) != 1 {
		t.Errorf("expected 1 target in group 0, got %d", len(dash.Groups[0].Targets))
	}
	if dash.Groups[1].Targets[0].Port != 1161 {
		t.Errorf("expected port 1161, got %d", dash.Groups[1].Targets[0].Port)
	}
}

func TestSaveDashboard(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "out.toml")

	dash := &Dashboard{
		Name:            "Saved Dashboard",
		DefaultIdentity: "prod-v3",
		Interval:        5 * time.Second,
		MaxHistory:      720,
		Groups: []Group{
			{Name: "Test", Targets: []Target{
				{Host: "1.2.3.4", Label: "test", Identity: "prod-v3", Interfaces: []string{"Eth0"}},
			}},
		},
	}
	if err := SaveDashboard(dash, path); err != nil {
		t.Fatalf("SaveDashboard() error: %v", err)
	}

	loaded, err := LoadDashboard(path)
	if err != nil {
		t.Fatalf("reload error: %v", err)
	}
	if loaded.Name != "Saved Dashboard" {
		t.Errorf("expected 'Saved Dashboard', got %q", loaded.Name)
	}
}

func TestListDashboards(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "a.toml"), []byte(testDashboardTOML), 0644)
	os.WriteFile(filepath.Join(tmp, "b.toml"), []byte(testDashboardTOML), 0644)
	os.WriteFile(filepath.Join(tmp, "not-toml.txt"), []byte("ignore"), 0644)

	names, err := ListDashboards(tmp)
	if err != nil {
		t.Fatalf("ListDashboards() error: %v", err)
	}
	if len(names) != 2 {
		t.Errorf("expected 2 dashboards, got %d", len(names))
	}
}
```

**Step 3: Run test to verify it fails**

Run: `go test ./internal/dashboard/ -v`
Expected: FAIL

**Step 4: Implement loader.go**

```go
package dashboard

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

func LoadDashboard(path string) (*Dashboard, error) {
	var dash Dashboard
	if _, err := toml.DecodeFile(path, &dash); err != nil {
		return nil, err
	}
	if dash.IntervalStr != "" {
		d, err := time.ParseDuration(dash.IntervalStr)
		if err == nil {
			dash.Interval = d
		}
	}
	if dash.Interval == 0 {
		dash.Interval = 10 * time.Second
	}
	if dash.MaxHistory == 0 {
		dash.MaxHistory = 360
	}
	for i := range dash.Groups {
		for j := range dash.Groups[i].Targets {
			if dash.Groups[i].Targets[j].Port == 0 {
				dash.Groups[i].Targets[j].Port = 161
			}
			if dash.Groups[i].Targets[j].Identity == "" {
				dash.Groups[i].Targets[j].Identity = dash.DefaultIdentity
			}
		}
	}
	return &dash, nil
}

func SaveDashboard(dash *Dashboard, path string) error {
	dash.IntervalStr = dash.Interval.String()
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(dash)
}

func ListDashboards(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".toml") {
			names = append(names, strings.TrimSuffix(e.Name(), ".toml"))
		}
	}
	return names, nil
}
```

**Step 5: Run tests**

Run: `go test ./internal/dashboard/ -v`
Expected: All PASS

**Step 6: Commit**

```
Add dashboard types and TOML loader with save/list support
```

---

### Task 8: Engine Poller & Manager

**Files:**
- Create: `internal/engine/poller.go`
- Create: `internal/engine/manager.go`
- Create: `internal/engine/types.go`
- Create: `internal/engine/discover.go`

**Step 1: Write types.go**

```go
package engine

import "time"

// InterfaceStats holds current monitoring data for a single interface.
type InterfaceStats struct {
	IfIndex     int
	Name        string
	Description string
	Speed       uint64 // Mbps
	Status      string // "up", "down", "testing"
	InRate      float64
	OutRate     float64
	Utilization float64
	History     *RingBuffer[RateSample]
	PollError   error
	LastPoll    time.Time
}

// TargetStats holds all interface stats for a single SNMP target.
type TargetStats struct {
	Host       string
	Label      string
	Interfaces []InterfaceStats
	PollError  error
	LastPoll   time.Time
}

// DashboardSnapshot is a point-in-time view of all dashboard data.
type DashboardSnapshot struct {
	Name      string
	Groups    []GroupSnapshot
	LastPoll  time.Time
	PollCount int
}

// GroupSnapshot is a point-in-time view of a group.
type GroupSnapshot struct {
	Name    string
	Targets []TargetStats
}

// EngineState represents the state of a dashboard engine.
type EngineState int

const (
	EngineStopped EngineState = iota
	EngineRunning
	EngineError
)

// EngineInfo provides metadata about a running engine.
type EngineInfo struct {
	Name      string
	State     EngineState
	LastPoll  time.Time
	PollCount int
	ErrorCount int
}

// EngineEvent is sent to subscribers when new data is available.
type EngineEvent struct {
	DashboardName string
	Snapshot      *DashboardSnapshot
}
```

**Step 2: Implement manager.go**

```go
package engine

import (
	"fmt"
	"sync"

	"github.com/tonhe/flo/internal/dashboard"
	"github.com/tonhe/flo/internal/identity"
)

// Manager coordinates multiple dashboard polling engines.
type Manager struct {
	mu      sync.RWMutex
	engines map[string]*Poller
}

func NewManager() *Manager {
	return &Manager{
		engines: make(map[string]*Poller),
	}
}

func (m *Manager) Start(dash *dashboard.Dashboard, provider identity.Provider) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.engines[dash.Name]; exists {
		return fmt.Errorf("engine %q already running", dash.Name)
	}
	p, err := NewPoller(dash, provider)
	if err != nil {
		return err
	}
	m.engines[dash.Name] = p
	go p.Run()
	return nil
}

func (m *Manager) Stop(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.engines[name]
	if !ok {
		return fmt.Errorf("engine %q not found", name)
	}
	p.Stop()
	delete(m.engines, name)
	return nil
}

func (m *Manager) GetSnapshot(name string) (*DashboardSnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.engines[name]
	if !ok {
		return nil, fmt.Errorf("engine %q not found", name)
	}
	return p.Snapshot(), nil
}

func (m *Manager) Subscribe(name string) (<-chan EngineEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.engines[name]
	if !ok {
		return nil, fmt.Errorf("engine %q not found", name)
	}
	return p.Subscribe(), nil
}

func (m *Manager) ListEngines() []EngineInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	infos := make([]EngineInfo, 0, len(m.engines))
	for _, p := range m.engines {
		infos = append(infos, p.Info())
	}
	return infos
}

func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for name, p := range m.engines {
		p.Stop()
		delete(m.engines, name)
	}
}
```

**Step 3: Implement poller.go**

This is the core polling loop. Each Poller runs as a goroutine, polling all targets in a dashboard at the configured interval.

```go
package engine

import (
	"sync"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/tonhe/flo/internal/dashboard"
	"github.com/tonhe/flo/internal/identity"
)

type Poller struct {
	mu          sync.RWMutex
	dash        *dashboard.Dashboard
	provider    identity.Provider
	clients     map[string]*gosnmp.GoSNMP // key: target host
	data        map[string]*TargetStats    // key: target host
	prevCounters map[string]map[int]CounterSample // host -> ifIndex -> sample
	subscribers []chan EngineEvent
	stopCh      chan struct{}
	pollCount   int
	errorCount  int
	lastPoll    time.Time
}

func NewPoller(dash *dashboard.Dashboard, provider identity.Provider) (*Poller, error) {
	p := &Poller{
		dash:         dash,
		provider:     provider,
		clients:      make(map[string]*gosnmp.GoSNMP),
		data:         make(map[string]*TargetStats),
		prevCounters: make(map[string]map[int]CounterSample),
		stopCh:       make(chan struct{}),
	}
	return p, nil
}

func (p *Poller) Run() {
	ticker := time.NewTicker(p.dash.Interval)
	defer ticker.Stop()

	// Initial poll
	p.poll()

	for {
		select {
		case <-ticker.C:
			p.poll()
		case <-p.stopCh:
			p.cleanup()
			return
		}
	}
}

func (p *Poller) poll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, group := range p.dash.Groups {
		for _, target := range group.Targets {
			p.pollTarget(group.Name, target)
		}
	}
	p.pollCount++
	p.lastPoll = time.Now()
	p.notify()
}

func (p *Poller) pollTarget(groupName string, target dashboard.Target) {
	client, err := p.getOrCreateClient(target)
	if err != nil {
		p.setTargetError(target, err)
		return
	}

	ts := p.getOrCreateTargetStats(target)
	now := time.Now()

	for i, iface := range ts.Interfaces {
		counters, err := p.getInterfaceCounters(client, iface.IfIndex)
		if err != nil {
			ts.Interfaces[i].PollError = err
			p.errorCount++
			continue
		}

		// Get status
		status, _ := p.getInterfaceStatus(client, iface.IfIndex)
		ts.Interfaces[i].Status = status

		// Calculate rate
		prevKey := target.Host
		if p.prevCounters[prevKey] != nil {
			if prev, ok := p.prevCounters[prevKey][iface.IfIndex]; ok {
				rate, err := CalculateRate(prev, counters)
				if err == nil {
					ts.Interfaces[i].InRate = rate.InRate
					ts.Interfaces[i].OutRate = rate.OutRate
					ts.Interfaces[i].Utilization = CalculateUtilization(rate.InRate, rate.OutRate, iface.Speed)
					ts.Interfaces[i].History.Add(rate)
				}
			}
		}

		if p.prevCounters[prevKey] == nil {
			p.prevCounters[prevKey] = make(map[int]CounterSample)
		}
		p.prevCounters[prevKey][iface.IfIndex] = counters
		ts.Interfaces[i].LastPoll = now
		ts.Interfaces[i].PollError = nil
	}
	ts.LastPoll = now
	ts.PollError = nil
}

func (p *Poller) getOrCreateClient(target dashboard.Target) (*gosnmp.GoSNMP, error) {
	if client, ok := p.clients[target.Host]; ok {
		return client, nil
	}
	id, err := p.provider.Get(target.Identity)
	if err != nil {
		return nil, err
	}
	client, err := NewSNMPClient(target.Host, target.Port, id, 5*time.Second)
	if err != nil {
		return nil, err
	}
	if err := client.Connect(); err != nil {
		return nil, err
	}
	p.clients[target.Host] = client
	return client, nil
}

func (p *Poller) getOrCreateTargetStats(target dashboard.Target) *TargetStats {
	if ts, ok := p.data[target.Host]; ok {
		return ts
	}
	ts := &TargetStats{
		Host:  target.Host,
		Label: target.Label,
	}
	for _, ifName := range target.Interfaces {
		ts.Interfaces = append(ts.Interfaces, InterfaceStats{
			Name:    ifName,
			History: NewRingBuffer[RateSample](p.dash.MaxHistory),
		})
	}
	p.data[target.Host] = ts
	return ts
}

func (p *Poller) getInterfaceCounters(client *gosnmp.GoSNMP, ifIndex int) (CounterSample, error) {
	oids := []string{
		fmt.Sprintf("%s.%d", OIDifHCInOctets, ifIndex),
		fmt.Sprintf("%s.%d", OIDifHCOutOctets, ifIndex),
	}
	result, err := client.Get(oids)
	if err != nil {
		return CounterSample{}, err
	}
	cs := CounterSample{Timestamp: time.Now()}
	for _, v := range result.Variables {
		val := gosnmp.ToBigInt(v.Value).Uint64()
		if v.Name == oids[0] || v.Name == "."+oids[0] {
			cs.InOctets = val
		} else {
			cs.OutOctets = val
		}
	}
	return cs, nil
}

func (p *Poller) getInterfaceStatus(client *gosnmp.GoSNMP, ifIndex int) (string, error) {
	oid := fmt.Sprintf("%s.%d", OIDifOperStatus, ifIndex)
	result, err := client.Get([]string{oid})
	if err != nil {
		return "unknown", err
	}
	if len(result.Variables) > 0 {
		val := gosnmp.ToBigInt(result.Variables[0].Value).Int64()
		switch val {
		case 1:
			return "up", nil
		case 2:
			return "down", nil
		case 3:
			return "testing", nil
		default:
			return "unknown", nil
		}
	}
	return "unknown", nil
}

func (p *Poller) setTargetError(target dashboard.Target, err error) {
	ts := p.getOrCreateTargetStats(target)
	ts.PollError = err
	p.errorCount++
}

func (p *Poller) Snapshot() *DashboardSnapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()

	snap := &DashboardSnapshot{
		Name:      p.dash.Name,
		LastPoll:  p.lastPoll,
		PollCount: p.pollCount,
	}
	for _, group := range p.dash.Groups {
		gs := GroupSnapshot{Name: group.Name}
		for _, target := range group.Targets {
			if ts, ok := p.data[target.Host]; ok {
				gs.Targets = append(gs.Targets, *ts)
			}
		}
		snap.Groups = append(snap.Groups, gs)
	}
	return snap
}

func (p *Poller) Subscribe() <-chan EngineEvent {
	ch := make(chan EngineEvent, 1)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.subscribers = append(p.subscribers, ch)
	return ch
}

func (p *Poller) notify() {
	snap := p.Snapshot()
	event := EngineEvent{DashboardName: p.dash.Name, Snapshot: snap}
	for _, ch := range p.subscribers {
		select {
		case ch <- event:
		default:
			// Drop if subscriber is behind
		}
	}
}

func (p *Poller) Info() EngineInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return EngineInfo{
		Name:       p.dash.Name,
		State:      EngineRunning,
		LastPoll:   p.lastPoll,
		PollCount:  p.pollCount,
		ErrorCount: p.errorCount,
	}
}

func (p *Poller) Stop() {
	close(p.stopCh)
}

func (p *Poller) cleanup() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, client := range p.clients {
		if client.Conn != nil {
			client.Conn.Close()
		}
	}
}
```

Note: Add the missing `fmt` import to poller.go.

**Step 4: Implement discover.go**

```go
package engine

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/tonhe/flo/internal/identity"
)

// DiscoveredInterface holds interface info from an SNMP walk.
type DiscoveredInterface struct {
	IfIndex     int
	Name        string
	Description string
	Alias       string
	Speed       uint64 // Mbps
	Status      string
}

// DiscoverInterfaces walks a device and returns its interfaces.
func DiscoverInterfaces(host string, port int, id *identity.Identity) ([]DiscoveredInterface, error) {
	client, err := NewSNMPClient(host, port, id, 10*time.Second)
	if err != nil {
		return nil, err
	}
	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("connect to %s: %w", host, err)
	}
	defer client.Conn.Close()

	interfaces := make(map[int]*DiscoveredInterface)

	// Walk ifName
	walkOID(client, OIDifName, func(idx int, val string) {
		if _, ok := interfaces[idx]; !ok {
			interfaces[idx] = &DiscoveredInterface{IfIndex: idx}
		}
		interfaces[idx].Name = val
	})

	// Walk ifDescr as fallback
	walkOID(client, OIDifDescr, func(idx int, val string) {
		if iface, ok := interfaces[idx]; ok && iface.Name == "" {
			iface.Name = val
		}
		if iface, ok := interfaces[idx]; ok {
			iface.Description = val
		}
	})

	// Walk ifAlias
	walkOID(client, OIDifAlias, func(idx int, val string) {
		if iface, ok := interfaces[idx]; ok {
			iface.Alias = val
		}
	})

	// Walk ifHighSpeed
	walkOID(client, OIDifHighSpeed, func(idx int, val string) {
		if iface, ok := interfaces[idx]; ok {
			speed, _ := strconv.ParseUint(val, 10, 64)
			iface.Speed = speed
		}
	})

	// Walk ifOperStatus
	walkOID(client, OIDifOperStatus, func(idx int, val string) {
		if iface, ok := interfaces[idx]; ok {
			v, _ := strconv.Atoi(val)
			switch v {
			case 1:
				iface.Status = "up"
			case 2:
				iface.Status = "down"
			default:
				iface.Status = "unknown"
			}
		}
	})

	result := make([]DiscoveredInterface, 0, len(interfaces))
	for _, iface := range interfaces {
		result = append(result, *iface)
	}
	return result, nil
}

func walkOID(client *gosnmp.GoSNMP, oid string, handler func(int, string)) {
	_ = client.BulkWalk(oid, func(pdu gosnmp.SnmpPDU) error {
		parts := strings.Split(pdu.Name, ".")
		if len(parts) == 0 {
			return nil
		}
		idx, err := strconv.Atoi(parts[len(parts)-1])
		if err != nil {
			return nil
		}
		var val string
		switch pdu.Type {
		case gosnmp.OctetString:
			val = string(pdu.Value.([]byte))
		default:
			val = fmt.Sprintf("%d", gosnmp.ToBigInt(pdu.Value))
		}
		handler(idx, val)
		return nil
	})
}
```

**Step 5: Build check**

Run: `go build ./...`
Expected: Build succeeds

**Step 6: Commit**

```
Add engine poller, manager, and device discovery
```

---

### Task 9: TUI Sparkline Component

**Files:**
- Create: `tui/components/sparkline.go`
- Create: `tui/components/sparkline_test.go`

**Step 1: Write sparkline_test.go**

```go
package components

import "testing"

func TestSparkline(t *testing.T) {
	data := []float64{0, 25, 50, 75, 100, 50, 25, 0}
	result := Sparkline(data, 8)
	if len([]rune(result)) != 8 {
		t.Errorf("expected 8 chars, got %d", len([]rune(result)))
	}
}

func TestSparklineEmpty(t *testing.T) {
	result := Sparkline(nil, 8)
	if result != "        " {
		t.Errorf("expected 8 spaces for empty data, got %q", result)
	}
}

func TestSparklineSingleValue(t *testing.T) {
	result := Sparkline([]float64{50}, 4)
	if len([]rune(result)) != 4 {
		t.Errorf("expected 4 chars, got %d", len([]rune(result)))
	}
}

func TestFormatRate(t *testing.T) {
	tests := []struct {
		bps      float64
		expected string
	}{
		{0, "0"},
		{500, "500b"},
		{1500, "1.5K"},
		{1_500_000, "1.5M"},
		{1_500_000_000, "1.5G"},
		{2_500_000_000_000, "2.5T"},
	}
	for _, tt := range tests {
		got := FormatRate(tt.bps)
		if got != tt.expected {
			t.Errorf("FormatRate(%f) = %q, want %q", tt.bps, got, tt.expected)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./tui/components/ -v`
Expected: FAIL

**Step 3: Implement sparkline.go**

```go
package components

import (
	"fmt"
	"strings"
)

var blocks = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// Sparkline renders a unicode block sparkline from data values.
// Returns a string of exactly `width` characters.
func Sparkline(data []float64, width int) string {
	if len(data) == 0 {
		return strings.Repeat(" ", width)
	}

	// Take last `width` values
	if len(data) > width {
		data = data[len(data)-width:]
	}

	// Find min/max
	min, max := data[0], data[0]
	for _, v := range data {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	// Pad left if fewer values than width
	var sb strings.Builder
	padding := width - len(data)
	for i := 0; i < padding; i++ {
		sb.WriteRune(' ')
	}

	spread := max - min
	for _, v := range data {
		if spread == 0 {
			sb.WriteRune(blocks[3]) // mid-level for flat data
		} else {
			normalized := (v - min) / spread
			idx := int(normalized * float64(len(blocks)-1))
			if idx >= len(blocks) {
				idx = len(blocks) - 1
			}
			sb.WriteRune(blocks[idx])
		}
	}
	return sb.String()
}

// FormatRate formats a bits-per-second value to a human-readable string.
func FormatRate(bps float64) string {
	if bps == 0 {
		return "0"
	}
	switch {
	case bps >= 1_000_000_000_000:
		return fmt.Sprintf("%.1fT", bps/1_000_000_000_000)
	case bps >= 1_000_000_000:
		return fmt.Sprintf("%.1fG", bps/1_000_000_000)
	case bps >= 1_000_000:
		return fmt.Sprintf("%.1fM", bps/1_000_000)
	case bps >= 1_000:
		return fmt.Sprintf("%.1fK", bps/1_000)
	default:
		return fmt.Sprintf("%.0fb", bps)
	}
}
```

**Step 4: Run tests**

Run: `go test ./tui/components/ -v`
Expected: All PASS

**Step 5: Commit**

```
Add sparkline renderer and rate formatter
```

---

### Task 10: TUI App Model & Main Dashboard View

This is the largest task - the main TUI shell and dashboard view.

**Files:**
- Create: `tui/app.go`
- Create: `tui/keys/keys.go`
- Create: `tui/views/dashboard.go`
- Create: `tui/components/header.go`
- Create: `tui/components/statusbar.go`
- Update: `main.go`

**Step 1: Implement keys.go**

```go
package keys

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Up        key.Binding
	Down      key.Binding
	Enter     key.Binding
	Escape    key.Binding
	Quit      key.Binding
	Dashboard key.Binding
	Identity  key.Binding
	New       key.Binding
	Settings  key.Binding
	Refresh   key.Binding
	Help      key.Binding
	Left      key.Binding
	Right     key.Binding
	Tab       key.Binding
}

var DefaultKeyMap = KeyMap{
	Up:        key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:      key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Enter:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
	Escape:    key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	Quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Dashboard: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "dashboards")),
	Identity:  key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "identities")),
	New:       key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new")),
	Settings:  key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "settings")),
	Refresh:   key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	Help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Left:      key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "left")),
	Right:     key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "right")),
	Tab:       key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next")),
}
```

**Step 2: Implement header.go and statusbar.go**

These are rendering-only components. Follow nbor's pattern of standalone render functions rather than full tea.Model implementations.

`tui/components/header.go`:
```go
package components

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/tonhe/flo/tui/styles"
)

func RenderHeader(theme styles.Theme, dashName string, isLive bool, activeCount, totalCount, width int) string {
	left := lipgloss.NewStyle().
		Foreground(theme.Base0D).
		Bold(true).
		Render("flo")

	center := lipgloss.NewStyle().
		Foreground(theme.Base05).
		Render(dashName)

	status := "STOPPED"
	statusColor := theme.Base08
	if isLive {
		status = "● LIVE"
		statusColor = theme.Base0B
	}
	right := lipgloss.NewStyle().
		Foreground(statusColor).
		Render(status)

	engines := lipgloss.NewStyle().
		Foreground(theme.Base04).
		Render(fmt.Sprintf("%d/%d active", activeCount, totalCount))

	content := fmt.Sprintf(" %s ─── %s ─── %s ── %s ", left, center, right, engines)

	return lipgloss.NewStyle().
		Background(theme.Base01).
		Width(width).
		Render(content)
}
```

`tui/components/statusbar.go`:
```go
package components

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/tonhe/flo/tui/styles"
)

func RenderStatusBar(theme styles.Theme, interval time.Duration, lastPoll time.Time, okCount, totalCount, width int) string {
	pollInfo := fmt.Sprintf("↻ %s", interval)
	lastStr := "never"
	if !lastPoll.IsZero() {
		lastStr = lastPoll.Format("15:04:05")
	}
	healthColor := theme.Base0B
	if okCount < totalCount {
		healthColor = theme.Base0A
	}

	stats := fmt.Sprintf("%s │ Last: %s │ ", pollInfo, lastStr)
	health := lipgloss.NewStyle().Foreground(healthColor).
		Render(fmt.Sprintf("%d/%d OK", okCount, totalCount))

	keyStyle := lipgloss.NewStyle().Foreground(theme.Base0D).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(theme.Base04)

	keys := fmt.Sprintf(
		" %s:%s  %s:%s  %s:%s  %s:%s  %s:%s  %s:%s  %s:%s",
		keyStyle.Render("enter"), descStyle.Render("detail"),
		keyStyle.Render("d"), descStyle.Render("dashboards"),
		keyStyle.Render("i"), descStyle.Render("identities"),
		keyStyle.Render("n"), descStyle.Render("new"),
		keyStyle.Render("s"), descStyle.Render("settings"),
		keyStyle.Render("?"), descStyle.Render("help"),
		keyStyle.Render("q"), descStyle.Render("quit"),
	)

	top := lipgloss.NewStyle().Background(theme.Base01).Width(width).
		Render(fmt.Sprintf(" %s%s", stats, health))
	bottom := lipgloss.NewStyle().Background(theme.Base01).Width(width).
		Render(keys)

	return lipgloss.JoinVertical(lipgloss.Left, top, bottom)
}
```

**Step 3: Implement dashboard view**

`tui/views/dashboard.go` - the main monitoring table. This is the hero screen. Follow nbor's pattern of a Model struct with Update/View, dynamic column sizing, and themed row rendering.

This file will be the most complex view - approximately 200-300 lines handling:
- Table rendering with group headers
- Row selection with cursor
- Sparkline rendering per interface
- Status coloring
- Rate formatting
- Utilization percentage coloring
- Responsive column widths

**Step 4: Implement app.go**

`tui/app.go` - root Bubble Tea model with state machine. Follow nbor's AppModel pattern: AppState enum, sub-models for each view, message routing in Update(), state-based View() rendering.

States: StateStartup, StateDashboard, StateSwitcher, StateDetail, StateIdentity, StateBuilder, StateSettings

**Step 5: Update main.go**

Wire up config loading, theme application, identity store, engine manager, and launch Bubble Tea with `tea.NewProgram()`.

**Step 6: Build and test manually**

Run: `go build -o flo . && ./flo`
Expected: TUI launches and shows the main dashboard view (empty initially)

**Step 7: Commit**

```
Add TUI app shell with dashboard view, header, and status bar
```

---

### Task 11: TUI Dashboard Switcher

**Files:**
- Create: `tui/views/switcher.go`

Implement as a floating overlay (modal) that lists all dashboards with their engine state. Follow the pattern from the design: `d` key opens it, shows LIVE/STOPPED status, allows start/stop/view/edit/delete.

**Commit:**
```
Add dashboard switcher overlay
```

---

### Task 12: TUI Detail View (Split-Screen)

**Files:**
- Create: `tui/views/dashboard_detail.go`
- Create: `tui/components/chart.go`

Implement the split-screen detail view activated by Enter. Top half shows the table (compressed), bottom half shows ASCII line charts for the selected interface's in/out traffic history. Chart renders from the ring buffer data.

**Commit:**
```
Add split-screen detail view with ASCII line charts
```

---

### Task 13: TUI Identity Manager

**Files:**
- Create: `tui/views/identity.go`

Full-screen view for CRUD operations on identities. Table display of identity summaries (never show secrets). Add/edit/test/delete operations via sub-modals with form inputs.

**Commit:**
```
Add identity manager TUI view
```

---

### Task 14: TUI Dashboard Builder Wizard

**Files:**
- Create: `tui/views/builder.go`

Step-by-step wizard: name + defaults, add targets with discovery, interface picker. Saves to TOML file in dashboards directory.

**Commit:**
```
Add dashboard builder wizard
```

---

### Task 15: TUI Settings View

**Files:**
- Create: `tui/views/settings.go`

Config editor with theme picker (live preview), default identity selector, and global settings. Follow nbor's config menu pattern with 2D grid navigation.

**Commit:**
```
Add settings view with theme picker
```

---

### Task 16: CLI Commands

**Files:**
- Create: `cmd/root.go`
- Create: `cmd/identity.go`
- Create: `cmd/discover.go`
- Create: `cmd/config.go`
- Update: `main.go`

Implement CLI subcommands: `flo identity add/list/test/remove`, `flo discover`, `flo config theme/default-identity/path`. Use Go's `flag` package or a simple command router (no heavy frameworks needed - keep it like nbor's `cli/` package).

**Commit:**
```
Add CLI commands for identity, discovery, and config management
```

---

### Task 17: Integration & Polish

**Files:**
- Update: `main.go` (wire everything together)
- Create: `tui/views/help.go` (help overlay showing all keybindings)

Final integration:
1. Master password prompt on startup (if identity store exists)
2. Dashboard auto-start from CLI flag `--dashboard`
3. Theme override from CLI flag `--theme`
4. Graceful shutdown (stop all engines, close connections)
5. Help overlay (`?` key)

**Commit:**
```
Wire up full application with startup flow and graceful shutdown
```

---

### Task 18: README & Final Verification

**Files:**
- Create: `README.md`

Write a polished README with:
- Feature overview
- Installation instructions (`go install`)
- Quick start guide
- Screenshot placeholders
- Configuration reference
- Theme list

Run full test suite:
Run: `go test ./... -v`
Expected: All PASS

Run build for all platforms:
Run: `GOOS=linux GOARCH=amd64 go build -o flo-linux . && GOOS=darwin GOARCH=arm64 go build -o flo-darwin . && GOOS=windows GOARCH=amd64 go build -o flo.exe .`

**Commit:**
```
Add README and verify cross-platform builds
```

---

## Execution Notes

- Tasks 1-9 are foundation layers with no TUI dependencies. They can be built and tested in isolation.
- Tasks 10-15 are TUI views. Task 10 is the critical path - all other views depend on the app shell.
- Task 16 (CLI) is independent of TUI views and can be parallelized.
- Each task should produce a working, testable increment.
- All tests should pass at every commit point.
