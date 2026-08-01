//go:build !windows

package packagemanager

type WindowsBackend struct{}

func NewWindowsBackend() *WindowsBackend {
	return &WindowsBackend{}
}

func (w *WindowsBackend) Name() string {
	return "windows"
}

func (w *WindowsBackend) IsSupported() bool {
	return false
}

func (w *WindowsBackend) ListPackages() ([]*Package, error) {
	return nil, nil
}
