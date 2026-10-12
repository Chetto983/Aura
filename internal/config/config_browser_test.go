package config

import "testing"

func TestBrowserSandboxConfigDefaultsOverridesAndMalformedKnobs(t *testing.T) {
	keys := []string{"AURA_SANDBOX_BROWSER_CPU_LIMIT", "AURA_SANDBOX_BROWSER_MEMORY_LIMIT", "AURA_SANDBOX_BROWSER_PIDS_LIMIT", "AURA_SANDBOX_BROWSER_MAX_ACTIVE"}
	for _, key := range keys {
		t.Setenv(key, "")
	}
	got := loadSandboxConfig()
	if got.BrowserCPULimit != 2 || got.BrowserMemoryLimit != 2147483648 || got.BrowserPidsLimit != 1024 || got.BrowserMaxActive != 0 {
		t.Fatalf("browser defaults: %+v", got)
	}
	for i, value := range []string{"3", "4294967296", "768", "2"} {
		t.Setenv(keys[i], value)
	}
	got = loadSandboxConfig()
	if got.BrowserCPULimit != 3 || got.BrowserMemoryLimit != 4294967296 || got.BrowserPidsLimit != 768 || got.BrowserMaxActive != 2 {
		t.Fatalf("browser overrides: %+v", got)
	}
	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, "malformed")
			if v, ok := findViolation(reparsePass(ProfileServerProduction), key); !ok || v.Sev != Fatal {
				t.Fatalf("malformed browser knob is not fatal in production: %+v, %t", v, ok)
			}
		})
	}
}
