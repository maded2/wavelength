package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"wavelength/internal/config"
)

func TestVoiceConfigIsEnabled(t *testing.T) {
	t.Run("is enabled by default when voice section is omitted", func(t *testing.T) {
		cfg := config.Config{}
		if !cfg.Voice.IsEnabled() {
			t.Error("expected voice to be enabled by default (auto-detect mode)")
		}
	})

	t.Run("is enabled when explicitly set to true", func(t *testing.T) {
		enabled := true
		cfg := config.Config{
			Voice: config.VoiceConfig{
				Enabled: &enabled,
			},
		}
		if !cfg.Voice.IsEnabled() {
			t.Error("expected voice to be enabled")
		}
	})

	t.Run("is disabled when explicitly set to false", func(t *testing.T) {
		disabled := false
		cfg := config.Config{
			Voice: config.VoiceConfig{
				Enabled: &disabled,
			},
		}
		if cfg.Voice.IsEnabled() {
			t.Error("expected voice to be disabled")
		}
	})
}

func TestVoiceConfigIsExplicitlyDisabled(t *testing.T) {
	t.Run("is not explicitly disabled when voice section is omitted", func(t *testing.T) {
		cfg := config.Config{}
		if cfg.Voice.IsExplicitlyDisabled() {
			t.Error("expected voice to not be explicitly disabled (should be auto-detect)")
		}
	})

	t.Run("is not explicitly disabled when set to true", func(t *testing.T) {
		enabled := true
		cfg := config.Config{
			Voice: config.VoiceConfig{
				Enabled: &enabled,
			},
		}
		if cfg.Voice.IsExplicitlyDisabled() {
			t.Error("expected voice to not be explicitly disabled")
		}
	})

	t.Run("is explicitly disabled when set to false", func(t *testing.T) {
		disabled := false
		cfg := config.Config{
			Voice: config.VoiceConfig{
				Enabled: &disabled,
			},
		}
		if !cfg.Voice.IsExplicitlyDisabled() {
			t.Error("expected voice to be explicitly disabled")
		}
	})
}

func TestVoiceConfigWhisperURL(t *testing.T) {
	t.Run("whisper_url is loaded from configuration", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "config.json")
		cfgJSON := `{
			"server": {"port": 3000},
			"llm": {
				"provider": "openai",
				"model": "gpt-4",
				"endpoint": "https://api.openai.com/v1",
				"api_key": "test-key"
			},
			"voice": {
				"whisper_url": "https://whisper.example.com/v1",
				"whisper_model": "whisper-large"
			},
			"data_dir": "./data"
		}`
		if err := os.WriteFile(cfgPath, []byte(cfgJSON), 0644); err != nil {
			t.Fatal(err)
		}

		cfg, err := config.Load(cfgPath)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if cfg.Voice.WhisperURL != "https://whisper.example.com/v1" {
			t.Errorf("expected whisper_url 'https://whisper.example.com/v1', got '%s'", cfg.Voice.WhisperURL)
		}
		if cfg.Voice.WhisperModel != "whisper-large" {
			t.Errorf("expected whisper_model 'whisper-large', got '%s'", cfg.Voice.WhisperModel)
		}
	})

	t.Run("whisper_url defaults to empty when not configured", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "config.json")
		cfgJSON := `{
			"server": {"port": 3000},
			"llm": {
				"provider": "openai",
				"model": "gpt-4",
				"endpoint": "https://api.openai.com/v1",
				"api_key": "test-key"
			},
			"data_dir": "./data"
		}`
		if err := os.WriteFile(cfgPath, []byte(cfgJSON), 0644); err != nil {
			t.Fatal(err)
		}

		cfg, err := config.Load(cfgPath)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if cfg.Voice.WhisperURL != "" {
			t.Errorf("expected empty whisper_url, got '%s'", cfg.Voice.WhisperURL)
		}
	})
}
