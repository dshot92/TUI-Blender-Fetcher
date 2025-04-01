package util

import (
	"fmt"
	// "math" // Removed unused import
)

// FormatSize converts bytes to a human-readable string (KB, MB, GB).
func FormatSize(sizeBytes int64) string {
	if sizeBytes < 1024 {
		return fmt.Sprintf("%d B", sizeBytes)
	}
	sizeKB := float64(sizeBytes) / 1024
	if sizeKB < 1024 {
		return fmt.Sprintf("%.1f KB", sizeKB)
	}
	sizeMB := sizeKB / 1024
	if sizeMB < 1024 {
		return fmt.Sprintf("%.1f MB", sizeMB)
	}
	sizeGB := sizeMB / 1024
	return fmt.Sprintf("%.1f GB", sizeGB)
}

// FormatSpeed converts bytes per second to a human-readable string (KB/s, MB/s, GB/s).
func FormatSpeed(bytesPerSecond float64) string {
	if bytesPerSecond < 1024 {
		return fmt.Sprintf("%.1f B/s", bytesPerSecond)
	}
	kbPerSecond := bytesPerSecond / 1024
	if kbPerSecond < 1024 {
		return fmt.Sprintf("%.1f KB/s", kbPerSecond)
	}
	mbPerSecond := kbPerSecond / 1024
	if mbPerSecond < 1024 {
		return fmt.Sprintf("%.1f MB/s", mbPerSecond)
	}
	gbPerSecond := mbPerSecond / 1024
	return fmt.Sprintf("%.1f GB/s", gbPerSecond)
}
