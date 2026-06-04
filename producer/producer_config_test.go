package producer

import (
	"os"
	"testing"

	"github.com/go-kit/kit/log"
)

func TestValidateMaxBatchSize(t *testing.T) {
	logger := log.NewLogfmtLogger(os.Stderr)

	tests := []struct {
		name     string
		input    int64
		expected int64
	}{
		{"negative resets to 5MB", -1, 1024 * 1024 * 5},
		{"zero resets to 5MB", 0, 1024 * 1024 * 5},
		{"valid small value kept", 1024, 1024},
		{"5MB kept", 1024 * 1024 * 5, 1024 * 1024 * 5},
		{"10MB kept", 1024 * 1024 * 10, 1024 * 1024 * 10},
		{"20MB kept", 1024 * 1024 * 20, 1024 * 1024 * 20},
		{"over 20MB clamped to 20MB", 1024*1024*20 + 1, 1024 * 1024 * 20},
		{"50MB clamped to 20MB", 1024 * 1024 * 50, 1024 * 1024 * 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := GetDefaultProducerConfig()
			config.MaxBatchSize = tt.input
			validated := validateProducerConfig(config, logger)
			if validated.MaxBatchSize != tt.expected {
				t.Errorf("MaxBatchSize = %d, want %d", validated.MaxBatchSize, tt.expected)
			}
		})
	}
}

func TestDefaultMaxBatchSize(t *testing.T) {
	config := GetDefaultProducerConfig()
	expected := int64(512 * 1024)
	if config.MaxBatchSize != expected {
		t.Errorf("default MaxBatchSize = %d, want %d", config.MaxBatchSize, expected)
	}
}
