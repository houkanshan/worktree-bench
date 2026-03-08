package version

var (
	// Version is set at build time via -ldflags.
	Version = "dev"
)

func String() string {
	return Version
}
