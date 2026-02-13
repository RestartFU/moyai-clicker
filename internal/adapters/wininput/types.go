package wininput

import "time"

type RuntimeConfig struct {
	TriggerCode  uint16
	ToggleCode   uint16
	CPS          float64
	ClickDown    time.Duration
	StartEnabled bool
}

type DeviceInfo struct {
	Path      string
	Name      string
	IsVirtual bool
	IsPointer bool
}
