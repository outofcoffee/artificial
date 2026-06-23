package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mustWrite writes content to path or fails the test.
func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestParseOutfit(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want selection
	}{
		{
			name: "provider and family",
			in:   "PROVIDER openrouter\nFAMILY deepseek-v4\n",
			want: selection{provider: "openrouter", family: "deepseek-v4"},
		},
		{
			name: "case-insensitive keywords",
			in:   "provider ollama\nFamily llama\n",
			want: selection{provider: "ollama", family: "llama"},
		},
		{
			name: "model only",
			in:   "PROVIDER llamacpp\nMODEL gemma-4-12b-it\n",
			want: selection{provider: "llamacpp", model: "gemma-4-12b-it"},
		},
		{
			name: "comments, blanks, and inline comments",
			in:   "# my Outfit\n\nPROVIDER openrouter   # the provider\nMODEL  m1\t# inline tab comment\n",
			want: selection{provider: "openrouter", model: "m1"},
		},
		{
			name: "extra whitespace and tabs as separator",
			in:   "PROVIDER\tollama\nFAMILY     llama\n",
			want: selection{provider: "ollama", family: "llama"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseOutfit([]byte(tc.in))
			if err != nil {
				t.Fatalf("parseOutfit: %v", err)
			}
			if got.provider != tc.want.provider || got.family != tc.want.family || got.model != tc.want.model {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestParseOutfit_Errors(t *testing.T) {
	cases := map[string]string{
		"missing provider":  "FAMILY llama\n",
		"unknown keyword":   "PROVIDER ollama\nFLAVOUR vanilla\n",
		"keyword no value":  "PROVIDER\n",
		"too many values":   "PROVIDER a b\n",
		"duplicate keyword": "PROVIDER a\nPROVIDER b\n",
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parseOutfit([]byte(in)); err == nil {
				t.Errorf("expected error for %q", in)
			}
		})
	}
}

func TestFormatOutfitRoundTrip(t *testing.T) {
	sel := selection{provider: "openrouter", family: "deepseek-v4", model: "deepseek/deepseek-v4-pro"}
	out := formatOutfit(sel)
	if !strings.HasPrefix(out, "PROVIDER openrouter\n") {
		t.Errorf("export not canonical:\n%s", out)
	}
	got, err := parseOutfit([]byte(out))
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if got != sel {
		t.Errorf("round-trip changed selection: %+v -> %+v", sel, got)
	}
}

func TestCmdApply_EndToEnd(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("DEEPSEEK_API_KEY", "sk-or-v1-test")

	outfit := filepath.Join(dir, "Outfit")
	mustWrite(t, outfit, "PROVIDER openrouter\nFAMILY deepseek-v4\n")

	out := captureStdout(t, func() {
		if err := cmdApply([]string{outfit}); err != nil {
			t.Fatalf("cmdApply: %v", err)
		}
	})
	if !strings.Contains(out, "Default model:") {
		t.Errorf("missing summary in output:\n%s", out)
	}

	m := readConfigMap(t, filepath.Join(dir, "opencode", "opencode.json"))
	if _, ok := m["provider"].(map[string]any)["openrouter"]; !ok {
		t.Error("openrouter provider not written")
	}
	if m["model"] != "openrouter/deepseek/deepseek-v4-flash" {
		t.Errorf("model = %v", m["model"])
	}
}

func TestCmdApply_DefaultFileMissing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Chdir(t.TempDir()) // a directory with no Outfit

	if err := cmdApply(nil); err == nil {
		t.Error("expected error when ./Outfit is missing")
	}
}

func TestCmdExport_RoundTripsWithApply(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("DEEPSEEK_API_KEY", "sk-or-v1-test")

	// Seed config via apply, then export it back out.
	outfit := filepath.Join(dir, "Outfit")
	mustWrite(t, outfit, "PROVIDER openrouter\nFAMILY deepseek-v4\n")
	captureStdout(t, func() {
		if err := cmdApply([]string{outfit}); err != nil {
			t.Fatalf("cmdApply: %v", err)
		}
	})

	out := captureStdout(t, func() {
		if err := cmdExport(nil); err != nil {
			t.Fatalf("cmdExport: %v", err)
		}
	})
	// The family's models match deepseek-v4, so export should name the family.
	if !strings.Contains(out, "PROVIDER openrouter") || !strings.Contains(out, "FAMILY   deepseek-v4") {
		t.Errorf("unexpected export:\n%s", out)
	}

	// And the exported Outfit must parse cleanly.
	if _, err := parseOutfit([]byte(out)); err != nil {
		t.Errorf("exported Outfit does not parse: %v", err)
	}
}
