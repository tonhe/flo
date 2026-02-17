package components

import (
	"fmt"
	"strings"
)

var blocks = []rune{'\u2581', '\u2582', '\u2583', '\u2584', '\u2585', '\u2586', '\u2587', '\u2588'}

func Sparkline(data []float64, width int) string {
	if len(data) == 0 {
		return strings.Repeat(" ", width)
	}
	if len(data) > width {
		data = data[len(data)-width:]
	}
	min, max := data[0], data[0]
	for _, v := range data {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	var sb strings.Builder
	padding := width - len(data)
	for i := 0; i < padding; i++ {
		sb.WriteRune(' ')
	}
	spread := max - min
	for _, v := range data {
		if spread == 0 {
			sb.WriteRune(blocks[3])
		} else {
			normalized := (v - min) / spread
			idx := int(normalized * float64(len(blocks)-1))
			if idx >= len(blocks) {
				idx = len(blocks) - 1
			}
			sb.WriteRune(blocks[idx])
		}
	}
	return sb.String()
}

func FormatRate(bps float64) string {
	if bps == 0 {
		return "0"
	}
	switch {
	case bps >= 1_000_000_000_000:
		return fmt.Sprintf("%.1fT", bps/1_000_000_000_000)
	case bps >= 1_000_000_000:
		return fmt.Sprintf("%.1fG", bps/1_000_000_000)
	case bps >= 1_000_000:
		return fmt.Sprintf("%.1fM", bps/1_000_000)
	case bps >= 1_000:
		return fmt.Sprintf("%.1fK", bps/1_000)
	default:
		return fmt.Sprintf("%.0fb", bps)
	}
}
