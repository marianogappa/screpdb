package appdata

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestResolveBase(t *testing.T) {
	userConfig := func(dir string, err error) func() (string, error) {
		return func() (string, error) { return dir, err }
	}
	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}

	tests := []struct {
		name       string
		goos       string
		env        map[string]string
		userConfig func() (string, error)
		want       string
		wantErr    bool
	}{
		{
			name: "windows uses LOCALAPPDATA not roaming",
			goos: "windows",
			env:  map[string]string{"LOCALAPPDATA": `C:\Users\bob\AppData\Local`},
			// Roaming would come from UserConfigDir; it must be ignored.
			userConfig: userConfig(`C:\Users\bob\AppData\Roaming`, nil),
			want:       `C:\Users\bob\AppData\Local`,
		},
		{
			name:       "darwin uses UserConfigDir",
			goos:       "darwin",
			env:        map[string]string{},
			userConfig: userConfig("/Users/bob/Library/Application Support", nil),
			want:       "/Users/bob/Library/Application Support",
		},
		{
			name:       "linux uses UserConfigDir",
			goos:       "linux",
			env:        map[string]string{},
			userConfig: userConfig("/home/bob/.config", nil),
			want:       "/home/bob/.config",
		},
		{
			name:       "non-windows propagates UserConfigDir error",
			goos:       "linux",
			env:        map[string]string{},
			userConfig: userConfig("", errors.New("no home")),
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveBase(tt.goos, env(tt.env), tt.userConfig)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (base %q)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("resolveBase() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveBaseWindowsFallsBackWhenNoLocalAppData(t *testing.T) {
	// With LOCALAPPDATA unset, Windows falls back to os.UserCacheDir (not the
	// injected UserConfigDir), so we only assert it does not return the roaming
	// dir and does not error out unexpectedly on the test host.
	got, err := resolveBase("windows", func(string) string { return "" }, func() (string, error) {
		return `C:\Users\bob\AppData\Roaming`, nil
	})
	if err == nil && got == `C:\Users\bob\AppData\Roaming` {
		t.Fatalf("windows must not fall back to roaming UserConfigDir, got %q", got)
	}
}

func TestResolveDBPath(t *testing.T) {
	t.Setenv(OverrideEnv, t.TempDir())

	def, err := DefaultDBPath()
	if err != nil {
		t.Fatalf("DefaultDBPath: %v", err)
	}
	if filepath.Base(def) != dbFileName {
		t.Errorf("default db base = %q, want %q", filepath.Base(def), dbFileName)
	}

	// Sentinel default resolves into the app-data root.
	got, err := ResolveDBPath(dbFileName)
	if err != nil {
		t.Fatalf("ResolveDBPath sentinel: %v", err)
	}
	if got != def {
		t.Errorf("sentinel resolve = %q, want %q", got, def)
	}

	// Empty resolves into the app-data root too.
	got, err = ResolveDBPath("  ")
	if err != nil {
		t.Fatalf("ResolveDBPath empty: %v", err)
	}
	if got != def {
		t.Errorf("empty resolve = %q, want %q", got, def)
	}

	// Explicit path is honored verbatim.
	explicit := "/tmp/custom/mine.db"
	got, err = ResolveDBPath(explicit)
	if err != nil {
		t.Fatalf("ResolveDBPath explicit: %v", err)
	}
	if got != explicit {
		t.Errorf("explicit resolve = %q, want %q", got, explicit)
	}
}

func TestOverrideEnvHonored(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(OverrideEnv, tmp)
	dir, err := Dir()
	if err != nil {
		t.Fatalf("Dir: %v", err)
	}
	if want, _ := filepath.Abs(tmp); dir != want {
		t.Errorf("Dir() = %q, want %q", dir, want)
	}
}
