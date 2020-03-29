package version

import (
	"fmt"
)

const (
	MajorVersion  = "1"
	MinorVersion  = "0"
	ReviseVersion = "6"
	BuildVersion  = "0319"
)

var (
	BuildTime = "None"
	GitHash   = "None"
	GoVersion = "None"
)

func GetVersion() string {
	return fmt.Sprintf("%s.%s.%s.%s", MajorVersion, MinorVersion, ReviseVersion, BuildVersion)
}
