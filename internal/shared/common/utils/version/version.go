package version

import "runtime"

var (
	Version   = "0.109.0"
	Commit    = "unknown"
	BuildTime = "unknown"
)

type Info struct {
	Version   string
	Commit    string
	BuildTime string
	GoVersion string
	Runtime   string
}

func Get() Info {
	return Info{
		Version:   Version,
		Commit:    Commit,
		BuildTime: BuildTime,
		GoVersion: runtime.Version(),
		Runtime:   Runtime(),
	}
}

func Runtime() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}

func String() string {
	info := Get()
	return "termbridge " + info.Version + " " + info.Runtime + " go=" + info.GoVersion + " commit=" + info.Commit + " built=" + info.BuildTime
}
