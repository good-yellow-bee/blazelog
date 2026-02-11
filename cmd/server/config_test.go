package main

import "testing"

func TestConfigValidate_AllowsExplicitInsecureMode(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Server.AllowInsecure = true
	cfg.Server.TLS.Enabled = false
	cfg.Server.HTTPTLS.Enabled = false

	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate config: %v", err)
	}
}

func TestConfigValidate_RejectsImplicitInsecureMode(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Server.AllowInsecure = false
	cfg.Server.TLS.Enabled = false
	cfg.Server.HTTPTLS.Enabled = false

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error when TLS is disabled without allow_insecure")
	}
}

func TestConfigValidate_RejectsInvalidAPIDurations(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Server.AllowInsecure = true
	cfg.API.MaxQueryRange = "not-a-duration"

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for invalid api.max_query_range")
	}
}
