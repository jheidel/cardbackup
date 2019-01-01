package util

import (
	"fmt"
	"time"
)

func BytesToString(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%c", float64(b)/float64(div), "KMGTPE"[exp])
}

func TruncateSeconds(d time.Duration) time.Duration {
	return time.Duration(d.Nanoseconds() / 1e9 * 1e9)
}
