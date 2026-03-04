package components

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func testColors() ChartColors {
	return ChartColors{
		BarFg:   lipgloss.Color("#00ff00"),
		LabelFg: lipgloss.Color("#888888"),
		TitleFg: lipgloss.Color("#0000ff"),
		Bg:      lipgloss.Color("#000000"),
	}
}

func TestRenderChartBasic(t *testing.T) {
	data := []float64{100, 200, 300, 400, 500}
	result := RenderChart(data, 30, 8, "Test", testColors())
	lines := strings.Split(result, "\n")
	if len(lines) != 8 {
		t.Errorf("expected 8 lines, got %d", len(lines))
	}
}

func TestRenderChartEmpty(t *testing.T) {
	result := RenderChart(nil, 30, 8, "Test", testColors())
	if result == "" {
		t.Error("expected non-empty output for nil data")
	}
}

func TestRenderChartWithOptions(t *testing.T) {
	now := time.Now()
	data := []float64{100, 200, 300, 400, 500}
	timestamps := make([]time.Time, len(data))
	for i := range timestamps {
		timestamps[i] = now.Add(time.Duration(i-len(data)+1) * 30 * time.Second)
	}

	opts := ChartOptions{
		Timestamps: timestamps,
		TimeFormat: "relative",
		Label:      "In",
	}
	result := RenderChartWithOptions(data, 40, 10, testColors(), opts)
	lines := strings.Split(result, "\n")
	if len(lines) != 10 {
		t.Errorf("expected 10 lines, got %d", len(lines))
	}
}

func TestRenderChartStatsTitle(t *testing.T) {
	data := []float64{1_000_000, 2_000_000, 3_000_000}
	opts := ChartOptions{
		Label: "In",
	}
	result := RenderChartWithOptions(data, 50, 8, testColors(), opts)
	if !strings.Contains(result, "In") {
		t.Error("expected title to contain label 'In'")
	}
}

func TestFormatTimeLabel(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		ts       time.Time
		format   string
		expected string
	}{
		{"relative_now", now, "relative", "now"},
		{"relative_5m", now.Add(-5 * time.Minute), "relative", "5m ago"},
		{"relative_1h", now.Add(-1 * time.Hour), "relative", "1h ago"},
		{"absolute", now, "absolute", now.Format("15:04")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatTimeLabel(tt.ts, now, tt.format)
			if got != tt.expected {
				t.Errorf("FormatTimeLabel() = %q, want %q", got, tt.expected)
			}
		})
	}
}
