package harness

import "testing"

func TestDefaultRegistryListsAdapters(t *testing.T) {
	items := DefaultRegistry().List()
	if len(items) < 4 {
		t.Fatalf("expected >=4 adapters, got %d", len(items))
	}
}

func TestAdapterLaunchEnvInjectsProxyURL(t *testing.T) {
	cfg, err := DefaultRegistry().Get(IDShellCommand)
	if err != nil {
		t.Fatal(err)
	}
	env := cfg.LaunchEnv("http://127.0.0.1:8787", map[string]string{"CWSO_HARNESS_PROMPT": "hi"})
	if env["OPENAI_BASE_URL"] != "http://127.0.0.1:8787" {
		t.Fatalf("OPENAI_BASE_URL = %q", env["OPENAI_BASE_URL"])
	}
	if env["CWSO_HARNESS_PROMPT"] != "hi" {
		t.Fatalf("prompt = %q", env["CWSO_HARNESS_PROMPT"])
	}
}

func TestRegistryRejectsUnknownAdapter(t *testing.T) {
	if _, err := DefaultRegistry().Get("missing"); err == nil {
		t.Fatal("expected error for unknown adapter")
	}
}
