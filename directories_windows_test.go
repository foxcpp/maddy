//go:build windows && !docker
// +build windows,!docker

package maddy

import (
	"path/filepath"
	"testing"
)

func TestWindowsDirectoryHelpers(t *testing.T) {
	t.Run("programData default fallback", func(t *testing.T) {
		t.Setenv("ProgramData", "")
		if got := programData(); got != defaultProgramData {
			t.Errorf("programData(): got %q, want %q", got, defaultProgramData)
		}
	})

	t.Run("programData from env", func(t *testing.T) {
		want := `D:\CustomData`
		t.Setenv("ProgramData", want)
		if got := programData(); got != want {
			t.Errorf("programData(): got %q, want %q", got, want)
		}
	})

	t.Run("programFiles default fallback", func(t *testing.T) {
		t.Setenv("ProgramFiles", "")
		if got := programFiles(); got != defaultProgramFiles {
			t.Errorf("programFiles(): got %q, want %q", got, defaultProgramFiles)
		}
	})

	t.Run("programFiles from env", func(t *testing.T) {
		want := `D:\CustomFiles`
		t.Setenv("ProgramFiles", want)
		if got := programFiles(); got != want {
			t.Errorf("programFiles(): got %q, want %q", got, want)
		}
	})

	t.Run("initialized directory variables", func(t *testing.T) {
		wantData := programData()
		wantFiles := programFiles()

		if got, want := ConfigDirectory, filepath.Join(wantData, `maddy`); got != want {
			t.Errorf("ConfigDirectory: got %q, want %q", got, want)
		}
		if got, want := DefaultStateDirectory, filepath.Join(wantData, `maddy`, `data`); got != want {
			t.Errorf("DefaultStateDirectory: got %q, want %q", got, want)
		}
		if got, want := DefaultRuntimeDirectory, filepath.Join(wantData, `maddy`, `run`); got != want {
			t.Errorf("DefaultRuntimeDirectory: got %q, want %q", got, want)
		}
		if got, want := DefaultLibexecDirectory, filepath.Join(wantFiles, `maddy`); got != want {
			t.Errorf("DefaultLibexecDirectory: got %q, want %q", got, want)
		}
	})
}
