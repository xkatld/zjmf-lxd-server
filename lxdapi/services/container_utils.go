package services

import (
	"fmt"
	"strings"
)

func getIntValue(newVal, oldVal int) int {
	if newVal > 0 {
		return newVal
	}
	return oldVal
}

func getMemoryValue(newVal, oldVal string) string {
	if newVal != "" {
		return newVal
	}
	return oldVal
}

func getStringValue(newVal, oldVal string) string {
	if newVal != "" {
		return newVal
	}
	return oldVal
}

func FormatBytes(bytes float64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%.0f B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	units := []string{"KB", "MB", "GB", "TB", "PB"}
	if exp >= len(units) {
		exp = len(units) - 1
	}

	result := bytes / float64(div)
	if result >= 1000 {
		if exp+1 < len(units) {
			result = result / unit
			exp++
		}
	}

	if result >= 100 {
		return fmt.Sprintf("%.0f %s", result, units[exp])
	} else if result >= 10 {
		return fmt.Sprintf("%.1f %s", result, units[exp])
	} else {
		return fmt.Sprintf("%.1f %s", result, units[exp])
	}
}

func FormatMemorySize(mb float64) string {
	if mb >= 1024 {
		gb := mb / 1024
		if gb >= 10 {
			return fmt.Sprintf("%.0f GB", gb)
		} else {
			return fmt.Sprintf("%.1f GB", gb)
		}
	}
	return fmt.Sprintf("%.0f MB", mb)
}

func FormatDiskSize(mb float64) string {
	gb := mb / 1024
	if gb >= 1024 {
		tb := gb / 1024
		if tb >= 10 {
			return fmt.Sprintf("%.0f TB", tb)
		} else {
			return fmt.Sprintf("%.1f TB", tb)
		}
	}
	if gb >= 10 {
		return fmt.Sprintf("%.0f GB", gb)
	} else {
		return fmt.Sprintf("%.1f GB", gb)
	}
}

func getBoolValue(newVal, oldVal bool) bool {
	return newVal
}

func ContainsNotFound(errMsg string) bool {
	notFoundIndicators := []string{
		"not found",
		"does not exist",
		"No such file or directory",
		"not exist",
	}

	for _, indicator := range notFoundIndicators {
		if ContainsIgnoreCase(errMsg, indicator) {
			return true
		}
	}
	return false
}

func ContainsIgnoreCase(s, substr string) bool {
	return strings.Contains(s, substr) || 
		   strings.Contains(strings.ToUpper(s), strings.ToUpper(substr)) || 
		   strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

