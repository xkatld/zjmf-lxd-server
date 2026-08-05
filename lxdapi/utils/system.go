package utils

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

type SystemInfo struct {
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	Kernel      string `json:"kernel"`
	Distribution string `json:"distribution"`
	LXDVersion  string `json:"lxd_version"`
}


func GetSystemInfo() SystemInfo {
	info := SystemInfo{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}


	if kernel := getKernelVersion(); kernel != "" {
		info.Kernel = kernel
	}


	if dist := getDistribution(); dist != "" {
		info.Distribution = dist
	}


	if lxdVer := getLXDVersion(); lxdVer != "" {
		info.LXDVersion = lxdVer
	}

	return info
}


func getKernelVersion() string {
	cmd := exec.Command("uname", "-r")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}


func getDistribution() string {

	cmd := exec.Command("sh", "-c", "grep '^PRETTY_NAME=' /etc/os-release | cut -d'=' -f2 | tr -d '\"'")
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		return strings.TrimSpace(string(output))
	}


	cmd = exec.Command("lsb_release", "-d", "-s")
	output, err = cmd.Output()
	if err == nil && len(output) > 0 {
		return strings.TrimSpace(string(output))
	}


	cmd = exec.Command("sh", "-c", "head -n1 /etc/issue | awk '{print $1, $2, $3}'")
	output, err = cmd.Output()
	if err == nil && len(output) > 0 {
		result := strings.TrimSpace(string(output))
		if result != "" && !strings.Contains(result, "\\") {
			return result
		}
	}

	return "unknown"
}


func getLXDVersion() string {

	cmd := exec.Command("lxc", "version")
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "Client version:") {
				parts := strings.Fields(line)
				if len(parts) >= 3 {
					return parts[2]
				}
			}
		}
	}


	cmd = exec.Command("lxd", "version")
	output, err = cmd.Output()
	if err == nil && len(output) > 0 {
		return strings.TrimSpace(string(output))
	}


	cmd = exec.Command("snap", "info", "lxd")
	output, err = cmd.Output()
	if err == nil && len(output) > 0 {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "installed:") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					return parts[1]
				}
			}
		}
	}

	return "unknown"
}


func GetSystemSummary() string {
	info := GetSystemInfo()

	summary := info.Distribution
	if summary == "unknown" || summary == "" {
		summary = strings.Title(info.OS)
	}

	if info.Arch != "" {
		summary += " (" + info.Arch + ")"
	}

	if info.Kernel != "" && info.Kernel != "unknown" {
		summary += " " + info.Kernel
	}

return summary
}


func ParseSizeToMB(sizeStr string) (int, error) {
	sizeStr = strings.ToUpper(strings.TrimSpace(sizeStr))
	if sizeStr == "" {
		return 0, nil
	}

	var multiplier int
	var valueStr string

	if strings.HasSuffix(sizeStr, "GB") {
		multiplier = 1024
		valueStr = strings.TrimSuffix(sizeStr, "GB")
	} else if strings.HasSuffix(sizeStr, "MB") {
		multiplier = 1
		valueStr = strings.TrimSuffix(sizeStr, "MB")
	} else if strings.HasSuffix(sizeStr, "TB") {
		multiplier = 1024 * 1024
		valueStr = strings.TrimSuffix(sizeStr, "TB")
	} else {

		multiplier = 1
		valueStr = sizeStr
	}

	value, err := strconv.Atoi(strings.TrimSpace(valueStr))
	if err != nil {
		return 0, fmt.Errorf("无效的尺寸值: %s", sizeStr)
	}

	return value * multiplier, nil
}
