package components

import "testing"

func TestSparkline(t *testing.T) {
	data := []float64{0, 25, 50, 75, 100, 50, 25, 0}
	result := Sparkline(data, 8)
	if len([]rune(result)) != 8 {
		t.Errorf("expected 8 chars, got %d", len([]rune(result)))
	}
}

func TestSparklineEmpty(t *testing.T) {
	result := Sparkline(nil, 8)
	if result != "        " {
		t.Errorf("expected 8 spaces for empty data, got %q", result)
	}
}

func TestSparklineSingleValue(t *testing.T) {
	result := Sparkline([]float64{50}, 4)
	if len([]rune(result)) != 4 {
		t.Errorf("expected 4 chars, got %d", len([]rune(result)))
	}
}

func TestFormatRate(t *testing.T) {
	tests := []struct {
		bps      float64
		expected string
	}{
		{0, "0"},
		{500, "500b"},
		{1500, "1.5K"},
		{1_500_000, "1.5M"},
		{1_500_000_000, "1.5G"},
		{2_500_000_000_000, "2.5T"},
	}
	for _, tt := range tests {
		got := FormatRate(tt.bps)
		if got != tt.expected {
			t.Errorf("FormatRate(%f) = %q, want %q", tt.bps, got, tt.expected)
		}
	}
}
