//go:build !linux && !windows

package main

import (
	"fmt"
	"log/slog"
	"strings"
	"time"
)

func parseTriggerCode(value string) (uint16, error) {
	return 0, fmt.Errorf("unsupported platform")
}

func parseBackendChoice(value string) (string, error) {
	backend := strings.ToLower(strings.TrimSpace(value))
	if backend == "" || backend == "auto" {
		return "auto", nil
	}
	return "", fmt.Errorf("invalid --backend %q (unsupported platform)", value)
}

func captureNextCode(_ string, _ string, timeout time.Duration) (uint16, error) {
	return 0, fmt.Errorf("unsupported platform")
}

func formatCodeName(code uint16) string {
	return fmt.Sprintf("%d", code)
}

func listInputDevices(_ string) error {
	return fmt.Errorf("input device listing is not supported on this platform")
}

func defaultGrabForTrigger(_ uint16) bool {
	return false
}

func permissionDeniedHint() string {
	return "Permission denied opening input backend."
}

func startClickerFromConfig(cfg config, logger *slog.Logger) (clickerRuntime, error) {
	return nil, fmt.Errorf("clicker runtime is not supported on this platform")
}
