package version

var (
	// Version is set at build time via -ldflags.
	Version = "v0.1.0"
)

func String() string {
	return Version
}
