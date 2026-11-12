//go:build !docker
// +build !docker

package maddy

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestDefaultDirectories(t *testing.T) {
	if ConfigDirectory == "" {
		t.Error("ConfigDirectory is empty")
	}
	if DefaultStateDirectory == "" {
		t.Error("DefaultStateDirectory is empty")
	}
	if DefaultRuntimeDirectory == "" {
		t.Error("DefaultRuntimeDirectory is empty")
	}
	if DefaultLibexecDirectory == "" {
		t.Error("DefaultLibexecDirectory is empty")
	}

	switch runtime.GOOS {
	case "linux":
		if ConfigDirectory != "/etc/maddy" {
			t.Errorf("ConfigDirectory on Linux: got %q, want %q", ConfigDirectory, "/etc/maddy")
		}
		if DefaultStateDirectory != "/var/lib/maddy" {
			t.Errorf("DefaultStateDirectory on Linux: got %q, want %q", DefaultStateDirectory, "/var/lib/maddy")
		}
		if DefaultRuntimeDirectory != "/run/maddy" {
			t.Errorf("DefaultRuntimeDirectory on Linux: got %q, want %q", DefaultRuntimeDirectory, "/run/maddy")
		}
		if DefaultLibexecDirectory != "/usr/lib/maddy" {
			t.Errorf("DefaultLibexecDirectory on Linux: got %q, want %q", DefaultLibexecDirectory, "/usr/lib/maddy")
		}
	case "darwin", "dragonfly", "freebsd", "netbsd", "openbsd":
		if ConfigDirectory != "/etc/maddy" {
			t.Errorf("ConfigDirectory on BSD/Darwin: got %q, want %q", ConfigDirectory, "/etc/maddy")
		}
		if DefaultStateDirectory != "/var/lib/maddy" {
			t.Errorf("DefaultStateDirectory on BSD/Darwin: got %q, want %q", DefaultStateDirectory, "/var/lib/maddy")
		}
		if DefaultRuntimeDirectory != "/var/run/maddy" {
			t.Errorf("DefaultRuntimeDirectory on BSD/Darwin: got %q, want %q", DefaultRuntimeDirectory, "/var/run/maddy")
		}
		if DefaultLibexecDirectory != "/usr/lib/maddy" {
			t.Errorf("DefaultLibexecDirectory on BSD/Darwin: got %q, want %q", DefaultLibexecDirectory, "/usr/lib/maddy")
		}
	case "windows":
		if !filepath.IsAbs(ConfigDirectory) {
			t.Errorf("ConfigDirectory on Windows should be absolute: got %q", ConfigDirectory)
		}
		if !filepath.IsAbs(DefaultStateDirectory) {
			t.Errorf("DefaultStateDirectory on Windows should be absolute: got %q", DefaultStateDirectory)
		}
		if !filepath.IsAbs(DefaultRuntimeDirectory) {
			t.Errorf("DefaultRuntimeDirectory on Windows should be absolute: got %q", DefaultRuntimeDirectory)
		}
		if !filepath.IsAbs(DefaultLibexecDirectory) {
			t.Errorf("DefaultLibexecDirectory on Windows should be absolute: got %q", DefaultLibexecDirectory)
		}
	}
}
