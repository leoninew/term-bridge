package git

type State string

const (
	StateAvailable             State = "available"
	StateNotRepository         State = "not_repository"
	StateExecutableUnavailable State = "executable_unavailable"
	StateBinary                State = "binary"
	StateTooLarge              State = "too_large"
	StateUnmerged              State = "unmerged"
	StateSubmodule             State = "submodule"
	StateLayerUnavailable      State = "layer_unavailable"
	StateUnavailable           State = "unavailable"
)

type Layer string

const (
	LayerStaged    Layer = "staged"
	LayerUnstaged  Layer = "unstaged"
	LayerUntracked Layer = "untracked"
)

type Change struct {
	Path            string
	OriginalPath    string
	IndexStatus     string
	WorktreeStatus  string
	Untracked       bool
	Unmerged        bool
	AvailableLayers []Layer
}

type StatusResult struct {
	State   State
	Changes []Change
	Message string
}

type DiffRequest struct {
	Path  string
	Layer Layer
}

type DiffResult struct {
	State        State
	OriginalPath string
	ModifiedPath string
	OriginalText string
	ModifiedText string
	Message      string
}
