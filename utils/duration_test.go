package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDuration_UnmarshalText(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{
			name:    "valid seconds",
			input:   "1s",
			want:    time.Second,
			wantErr: false,
		},
		{
			name:    "valid minutes",
			input:   "5m",
			want:    5 * time.Minute,
			wantErr: false,
		},
		{
			name:    "valid hours",
			input:   "1h",
			want:    time.Hour,
			wantErr: false,
		},
		{
			name:    "valid complex duration",
			input:   "1h30m15s",
			want:    time.Hour + 30*time.Minute + 15*time.Second,
			wantErr: false,
		},
		{
			name:    "valid milliseconds",
			input:   "500ms",
			want:    500 * time.Millisecond,
			wantErr: false,
		},
		{
			name:    "invalid duration",
			input:   "invalid",
			want:    0,
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d Duration
			err := d.UnmarshalText([]byte(tt.input))
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, d.Duration)
			}
		})
	}
}

func TestDuration_MarshalText(t *testing.T) {
	tests := []struct {
		name     string
		duration Duration
		want     string
		wantErr  bool
	}{
		{
			name:     "one second",
			duration: Duration{Duration: time.Second},
			want:     "1s",
			wantErr:  false,
		},
		{
			name:     "five minutes",
			duration: Duration{Duration: 5 * time.Minute},
			want:     "5m0s",
			wantErr:  false,
		},
		{
			name:     "one hour",
			duration: Duration{Duration: time.Hour},
			want:     "1h0m0s",
			wantErr:  false,
		},
		{
			name:     "complex duration",
			duration: Duration{Duration: time.Hour + 30*time.Minute + 15*time.Second},
			want:     "1h30m15s",
			wantErr:  false,
		},
		{
			name:     "zero duration",
			duration: Duration{Duration: 0},
			want:     "0s",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.duration.MarshalText()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, string(got))
			}
		})
	}
}

func TestDuration_RoundTrip(t *testing.T) {
	tests := []string{
		"1s",
		"5m",
		"1h",
		"1h30m15s",
		"500ms",
		"24h",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			var d Duration
			err := d.UnmarshalText([]byte(input))
			require.NoError(t, err)

			marshaled, err := d.MarshalText()
			require.NoError(t, err)

			// Parse the marshaled text back
			var d2 Duration
			err = d2.UnmarshalText(marshaled)
			require.NoError(t, err)

			// The durations should be equal (allowing for Go's duration formatting)
			assert.Equal(t, d.Duration, d2.Duration)
		})
	}
}
