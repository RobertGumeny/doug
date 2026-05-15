package build

// StaticBuildSystem implements BuildSystem for plain HTML/CSS/JS projects
// with no build step. All operations are no-ops and the project is always
// considered initialized.
type StaticBuildSystem struct{}

// IsInitialized always returns true — static projects need no setup.
func (s *StaticBuildSystem) IsInitialized() bool { return true }

// Install is a no-op for static projects.
func (s *StaticBuildSystem) Install() error { return nil }

// Build is a no-op for static projects.
func (s *StaticBuildSystem) Build() error { return nil }

// Test is a no-op for static projects.
func (s *StaticBuildSystem) Test() error { return nil }

// Lint is a no-op for static projects.
func (s *StaticBuildSystem) Lint() error { return nil }
