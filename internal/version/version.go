package version

// Version is the build version, injected at build time via
// -ldflags "-X plusplus/internal/version.Version=...".
// It defaults to "dev" for local builds.
var Version = "dev"
