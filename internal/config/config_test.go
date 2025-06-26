package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		wantErr bool
	}{
		{
			name:    "default config",
			envVars: map[string]string{},
			wantErr: false,
		},
		{
			name: "custom config",
			envVars: map[string]string{
				"SERVICE_NAME":     "test-service",
				"LISTEN_PORT":      "4444",
				"POOL_FEE_PERCENT": "2.5",
				"MIN_DIFFICULTY":   "2.0",
			},
			wantErr: false,
		},
		{
			name: "invalid port",
			envVars: map[string]string{
				"LISTEN_PORT": "99999",
			},
			wantErr: true,
		},
		{
			name: "invalid fee",
			envVars: map[string]string{
				"POOL_FEE_PERCENT": "150",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for key, value := range tt.envVars {
				if err := os.Setenv(key, value); err != nil {
					t.Fatalf("failed to set environment variable %s: %v", key, err)
				}
			}
			defer func() {
				// Clean up environment variables
				for key := range tt.envVars {
					if err := os.Unsetenv(key); err != nil {
						t.Logf("failed to unset environment variable %s: %v", key, err)
					}
				}
			}()

			cfg, err := Load()
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify some basic fields
				if cfg.ServiceName == "" {
					t.Error("ServiceName should not be empty")
				}
				if cfg.ListenPort <= 0 {
					t.Error("ListenPort should be positive")
				}
			}
		})
	}
}

func TestConfigValidation(t *testing.T) {
	cfg := &Config{
		ServiceName:       "test",
		ListenPort:        3333,
		PoolFeePercent:    1.0,
		PPLNSWindowFactor: 2.0,
		MinDifficulty:     1.0,
		MaxDifficulty:     1000.0,
	}

	if err := cfg.validate(); err != nil {
		t.Errorf("validate() should not fail for valid config: %v", err)
	}

	// Test invalid configurations
	invalidConfigs := []*Config{
		{ServiceName: "", ListenPort: 3333, PoolFeePercent: 1.0, PPLNSWindowFactor: 2.0, MinDifficulty: 1.0, MaxDifficulty: 1000.0},
		{ServiceName: "test", ListenPort: 0, PoolFeePercent: 1.0, PPLNSWindowFactor: 2.0, MinDifficulty: 1.0, MaxDifficulty: 1000.0},
		{ServiceName: "test", ListenPort: 3333, PoolFeePercent: 150.0, PPLNSWindowFactor: 2.0, MinDifficulty: 1.0, MaxDifficulty: 1000.0},
		{ServiceName: "test", ListenPort: 3333, PoolFeePercent: 1.0, PPLNSWindowFactor: 0, MinDifficulty: 1.0, MaxDifficulty: 1000.0},
		{ServiceName: "test", ListenPort: 3333, PoolFeePercent: 1.0, PPLNSWindowFactor: 2.0, MinDifficulty: 0, MaxDifficulty: 1000.0},
		{ServiceName: "test", ListenPort: 3333, PoolFeePercent: 1.0, PPLNSWindowFactor: 2.0, MinDifficulty: 1000.0, MaxDifficulty: 1.0},
	}

	for i, cfg := range invalidConfigs {
		if err := cfg.validate(); err == nil {
			t.Errorf("validate() should fail for invalid config %d", i)
		}
	}
}

func TestGetEnvHelpers(t *testing.T) {
	// Test getEnv
	if err := os.Setenv("TEST_STRING", "test_value"); err != nil {
		t.Fatalf("failed to set TEST_STRING: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("TEST_STRING"); err != nil {
			t.Logf("failed to unset TEST_STRING: %v", err)
		}
	}()

	if got := getEnv("TEST_STRING", "default"); got != "test_value" {
		t.Errorf("getEnv() = %v, want %v", got, "test_value")
	}

	if got := getEnv("NONEXISTENT", "default"); got != "default" {
		t.Errorf("getEnv() = %v, want %v", got, "default")
	}

	// Test getEnvInt
	if err := os.Setenv("TEST_INT", "42"); err != nil {
		t.Fatalf("failed to set TEST_INT: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("TEST_INT"); err != nil {
			t.Logf("failed to unset TEST_INT: %v", err)
		}
	}()

	if got := getEnvInt("TEST_INT", 0); got != 42 {
		t.Errorf("getEnvInt() = %v, want %v", got, 42)
	}

	if got := getEnvInt("NONEXISTENT", 99); got != 99 {
		t.Errorf("getEnvInt() = %v, want %v", got, 99)
	}

	// Test getEnvFloat
	if err := os.Setenv("TEST_FLOAT", "3.14"); err != nil {
		t.Fatalf("failed to set TEST_FLOAT: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("TEST_FLOAT"); err != nil {
			t.Logf("failed to unset TEST_FLOAT: %v", err)
		}
	}()

	if got := getEnvFloat("TEST_FLOAT", 0.0); got != 3.14 {
		t.Errorf("getEnvFloat() = %v, want %v", got, 3.14)
	}

	// Test getEnvDuration
	if err := os.Setenv("TEST_DURATION", "30s"); err != nil {
		t.Fatalf("failed to set TEST_DURATION: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("TEST_DURATION"); err != nil {
			t.Logf("failed to unset TEST_DURATION: %v", err)
		}
	}()

	if got := getEnvDuration("TEST_DURATION", 0); got != 30*time.Second {
		t.Errorf("getEnvDuration() = %v, want %v", got, 30*time.Second)
	}
}
