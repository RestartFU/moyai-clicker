//go:build !windows

package wininput

import (
	"fmt"
	"time"

	"clicker/internal/core/autoclicker"
)

type Runtime struct{}

func NewRuntime(cfg RuntimeConfig, logger autoclicker.Logger) (*Runtime, error) {
	return nil, fmt.Errorf("windows input runtime is only available on Windows")
}

func (r *Runtime) Start() error {
	return fmt.Errorf("windows input runtime is only available on Windows")
}

func (r *Runtime) Stop() {}

func (r *Runtime) SetEnabled(enabled bool) {}

func (r *Runtime) IsEnabled() bool {
	return false
}

func (r *Runtime) SetCPS(cps float64) error {
	return fmt.Errorf("windows input runtime is only available on Windows")
}

func (r *Runtime) SetTriggerCode(code uint16) {}

func (r *Runtime) SetToggleCode(code uint16) {}

func (r *Runtime) CaptureNextKeyCode(timeout time.Duration) (uint16, error) {
	return 0, fmt.Errorf("windows input runtime is only available on Windows")
}

func ListInputDevices() ([]DeviceInfo, error) {
	return nil, fmt.Errorf("windows input runtime is only available on Windows")
}

func CaptureNextKeyCode(timeout time.Duration) (uint16, error) {
	return 0, fmt.Errorf("windows input runtime is only available on Windows")
}
