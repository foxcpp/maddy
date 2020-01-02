//+build !linux

package maddy

type SDStatus string

const (
	SDReady     = "READY=1"
	SDReloading = "RELOADING=1"
	SDStopping  = "STOPPING=1"
)

func systemdStatus(SDStatus, string) {}

func systemdStatusErr(error) {}
