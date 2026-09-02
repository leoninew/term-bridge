package version

import "runtime"

var (
	Version = "0.116.1"
)

type Info struct {
	Version   string
	GoVersion string
	Runtime   string
}

func Get() Info {
	return Info{
		Version:   Version,
		GoVersion: runtime.Version(),
		Runtime:   Runtime(),
	}
}

func Runtime() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}

func String() string {
	info := Get()
	return "termbridge " + info.Version + " " + info.Runtime + " go=" + info.GoVersion
}
