package version

const DefaultVersion = "0.1.0"

var (
	Version = DefaultVersion
	Commit  = "unknown"
	Date    = "unknown"
)

func String() string {
	return Version
}
