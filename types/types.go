package types

// BuildState represents the current state of a Blender build
type BuildState int

const (
	// StateNone is the default state
	StateNone BuildState = iota
	// StateInstalled indicates the build is installed locally
	StateInstalled
	// StateDownloading indicates the build is currently being downloaded
	StateDownloading
	// StatePreparing indicates the build is being prepared for extraction
	StatePreparing
	// StateExtracting indicates the build is being extracted
	StateExtracting
	// StateRunning indicates Blender is currently running
	StateRunning
	// StateLocal indicates the build exists locally
	StateLocal
	// StateOnline indicates the build is available online
	StateOnline
	// StateUpdate indicates a newer version is available online
	StateUpdate
	// StateFailed indicates a failed operation
	StateFailed
)

// String returns the string representation of the BuildState
func (s BuildState) String() string {
	switch s {
	case StateNone:
		return "Cancelled"
	case StateInstalled:
		return "Installed"
	case StateDownloading:
		return "Downloading"
	case StatePreparing:
		return "Preparing"
	case StateExtracting:
		return "Extracting"
	case StateRunning:
		return "Running"
	case StateLocal:
		return "Local"
	case StateOnline:
		return "Online"
	case StateUpdate:
		return "Update"
	case StateFailed:
		return "Failed"
	default:
		return "Unknown"
	}
}
