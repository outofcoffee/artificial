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

func TestCmdExport_NoProviders(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := cmdExport(nil); err == nil {
		t.Error("expected error when nothing is configured")
	}
}

// TestCmdExport_ModelOnlyFallsBackToModel covers a provider whose configured
// models match no known family (a bare llama.cpp label): export should still
// produce a valid Outfit naming the MODEL.
func TestCmdExport_ModelOnlyFallsBackToModel(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	outfit := filepath.Join(dir, "Outfit")
	mustWrite(t, outfit, "PROVIDER llamacpp\nMODEL my-local-model\n")
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
	if !strings.Contains(out, "PROVIDER llamacpp") || !strings.Contains(out, "MODEL    my-local-model") {
		t.Errorf("unexpected export:\n%s", out)
	}
	if strings.Contains(out, "FAMILY") {
		t.Errorf("did not expect a FAMILY line for an unrecognised model:\n%s", out)
	}
}

// TestCmdExport_FamilyPlusNonDefaultModel checks that when the default model is
// not the family's own default, export keeps both the FAMILY and the MODEL.
func TestCmdExport_FamilyPlusNonDefaultModel(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("DEEPSEEK_API_KEY", "sk-or-v1-test")

	cat, _ := loadCatalog()
	fam := cat.Providers["openrouter"].Families["deepseek-v4"]
	// Find a model in the family that is not its default.
	var nonDefault string
	for _, k := range fam.modelKeys() {
		if k != fam.DefaultModel {
			nonDefault = k
			break
		}
	}
	if nonDefault == "" {
		t.Skip("family has no non-default model to exercise this path")
	}

	outfit := filepath.Join(dir, "Outfit")
	mustWrite(t, outfit, "PROVIDER openrouter\nFAMILY deepseek-v4\nMODEL "+nonDefault+"\n")
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
	if !strings.Contains(out, "FAMILY   deepseek-v4") || !strings.Contains(out, "MODEL    "+nonDefault) {
		t.Errorf("expected both FAMILY and the non-default MODEL:\n%s", out)
	}
}

// TestCmdExport_MultipleProviders covers provider selection when several are
// configured: without a hint it errors, with -p it exports the chosen one, and
// an unknown -p errors.
func TestCmdExport_MultipleProviders(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	// Seed two providers with no default model, so neither is implied.
	path, _ := resolveConfigFile()
	cat, _ := loadCatalog()
	for _, id := range []string{"ollama", "llamacpp"} {
		block, _, err := buildProviderBlock(id, cat.Providers[id], "", "label-"+id, "", noEnv)
		if err != nil {
			t.Fatal(err)
		}
		if err := writeConfig(path, id, block, ""); err != nil {
			t.Fatal(err)
		}
	}

	if err := cmdExport(nil); err == nil {
		t.Error("expected error when several providers are configured and none is implied")
	}

	out := captureStdout(t, func() {
		if err := cmdExport([]string{"-p", "ollama"}); err != nil {
			t.Fatalf("cmdExport -p ollama: %v", err)
		}
	})
	if !strings.Contains(out, "PROVIDER ollama") {
		t.Errorf("export -p ollama gave:\n%s", out)
	}

	if err := cmdExport([]string{"-p", "nonesuch"}); err == nil {
		t.Error("expected error for a provider that is not configured")
	}
}

func TestCmdApply_BadOutfitContent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	bad := filepath.Join(dir, "Outfit")
	mustWrite(t, bad, "FAMILY llama\n") // no PROVIDER
	if err := cmdApply([]string{bad}); err == nil {
		t.Error("expected error for an Outfit without a PROVIDER")
	}
}

func TestCmdApply_MissingExplicitPath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := cmdApply([]string{filepath.Join(t.TempDir(), "nope.outfit")}); err == nil {
		t.Error("expected error for a missing explicit path")
	}
}
