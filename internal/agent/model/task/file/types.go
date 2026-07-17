package file

import "time"

type EntryKind string

const (
	EntryKindFile      EntryKind = "file"
	EntryKindDirectory EntryKind = "directory"
)

type Entry struct {
	Path        RelativePath
	Name        string
	Kind        EntryKind
	Size        int64
	ModifiedAt  time.Time
	Revision    string
	HasChildren *bool
}

type ListResult struct {
	Directory Entry
	Items     []Entry
	Truncated bool
}

type ReadResult struct {
	Entry Entry
	Text  string
}

type MutationResult struct {
	SourceEntry          *Entry
	DestinationEntry     *Entry
	AffectedCount        int
	AffectedPathPrefixes []RelativePath
}

type CreateFileRequest struct {
	Path                        RelativePath
	Text                        string
	Overwrite                   bool
	ExpectedParentRevision      string
	ExpectedDestinationRevision string
}

type CreateDirectoryRequest struct {
	Path                   RelativePath
	AllowExisting          bool
	ExpectedParentRevision string
}

type WriteRequest struct {
	Path             RelativePath
	Text             string
	ExpectedRevision string
	Force            bool
}

type RenameRequest struct {
	Path                        RelativePath
	NewName                     string
	ExpectedSourceRevision      string
	ExpectedParentRevision      string
	ExpectedDestinationRevision string
}

type MoveRequest struct {
	SourcePath                        RelativePath
	DestinationPath                   RelativePath
	Overwrite                         bool
	ExpectedSourceRevision            string
	ExpectedSourceParentRevision      string
	ExpectedDestinationParentRevision string
	ExpectedDestinationRevision       string
}

type DeleteRequest struct {
	Path                   RelativePath
	ExpectedRevision       string
	ExpectedParentRevision string
	Recursive              bool
}
