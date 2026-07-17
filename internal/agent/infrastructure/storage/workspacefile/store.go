package workspacefile

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	filemodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/file"
)

type Config struct {
	MaxTextBytes              int64
	MaxDirectoryEntries       int
	MaxRecursiveDeleteEntries int
}

type Store struct {
	config Config
	locks  sync.Map
}

func New(config Config) (*Store, error) {
	if config.MaxTextBytes < 1 || config.MaxDirectoryEntries < 1 || config.MaxRecursiveDeleteEntries < 1 {
		return nil, errors.New("workspace file limits must be positive")
	}
	return &Store{config: config}, nil
}

func (s *Store) List(ctx context.Context, rootPath string, directory filemodel.RelativePath) (filemodel.ListResult, error) {
	root, err := openRoot(rootPath)
	if err != nil {
		return filemodel.ListResult{}, err
	}
	defer func() { _ = root.Close() }()
	if err := ctx.Err(); err != nil {
		return filemodel.ListResult{}, err
	}
	dirEntry, err := root.entry(directory)
	if err != nil {
		return filemodel.ListResult{}, err
	}
	if dirEntry.Kind != filemodel.EntryKindDirectory {
		return filemodel.ListResult{}, filemodel.NewError("type_mismatch", "The path is not a directory.")
	}
	items, err := root.list(directory, s.config.MaxDirectoryEntries)
	if err != nil {
		return filemodel.ListResult{}, err
	}
	return filemodel.ListResult{Directory: dirEntry, Items: items.items, Truncated: items.truncated}, nil
}

func (s *Store) Stat(ctx context.Context, rootPath string, path filemodel.RelativePath) (filemodel.Entry, error) {
	root, err := openRoot(rootPath)
	if err != nil {
		return filemodel.Entry{}, err
	}
	defer func() { _ = root.Close() }()
	if err := ctx.Err(); err != nil {
		return filemodel.Entry{}, err
	}
	return root.entry(path)
}

func (s *Store) Read(ctx context.Context, rootPath string, path filemodel.RelativePath) (filemodel.ReadResult, error) {
	root, err := openRoot(rootPath)
	if err != nil {
		return filemodel.ReadResult{}, err
	}
	defer func() { _ = root.Close() }()
	if err := ctx.Err(); err != nil {
		return filemodel.ReadResult{}, err
	}
	entry, err := root.entry(path)
	if err != nil {
		return filemodel.ReadResult{}, err
	}
	if entry.Kind != filemodel.EntryKindFile {
		return filemodel.ReadResult{}, filemodel.NewError("type_mismatch", "The path is not a file.")
	}
	data, err := readBounded(root, path, s.config.MaxTextBytes)
	if err != nil {
		return filemodel.ReadResult{}, err
	}
	return filemodel.ReadResult{Entry: entry, Text: string(data)}, nil
}

func (s *Store) CreateFile(ctx context.Context, rootPath string, request filemodel.CreateFileRequest) (filemodel.MutationResult, error) {
	return s.withLock(ctx, rootPath, func(root *root) (filemodel.MutationResult, error) {
		if err := validateText(request.Text, s.config.MaxTextBytes); err != nil {
			return filemodel.MutationResult{}, err
		}
		if err := root.requireRevision(request.Path.Parent(), request.ExpectedParentRevision); err != nil {
			return filemodel.MutationResult{}, err
		}
		if existing, err := root.entry(request.Path); err == nil {
			if !request.Overwrite {
				return filemodel.MutationResult{}, filemodel.NewError("already_exists", "The destination already exists.")
			}
			if existing.Kind != filemodel.EntryKindFile {
				return filemodel.MutationResult{}, filemodel.NewError("type_mismatch", "Only a regular text file can be overwritten.")
			}
			if err := requireEntryRevision(existing, request.ExpectedDestinationRevision); err != nil {
				return filemodel.MutationResult{}, err
			}
			return root.write(request.Path, request.Text, existing, request.Path.Parent())
		} else if !isNotFound(err) {
			return filemodel.MutationResult{}, err
		}
		if err := root.create(request.Path, request.Text); err != nil {
			return filemodel.MutationResult{}, err
		}
		created, err := root.entry(request.Path)
		if err != nil {
			return filemodel.MutationResult{}, err
		}
		return filemodel.MutationResult{DestinationEntry: &created, AffectedCount: 1, AffectedPathPrefixes: []filemodel.RelativePath{request.Path}}, nil
	})
}

func (s *Store) CreateDirectory(ctx context.Context, rootPath string, request filemodel.CreateDirectoryRequest) (filemodel.MutationResult, error) {
	return s.withLock(ctx, rootPath, func(root *root) (filemodel.MutationResult, error) {
		if err := root.requireRevision(request.Path.Parent(), request.ExpectedParentRevision); err != nil {
			return filemodel.MutationResult{}, err
		}
		if existing, err := root.entry(request.Path); err == nil {
			if request.AllowExisting && existing.Kind == filemodel.EntryKindDirectory {
				return filemodel.MutationResult{DestinationEntry: &existing, AffectedCount: 0}, nil
			}
			return filemodel.MutationResult{}, filemodel.NewError("already_exists", "The destination already exists.")
		} else if !isNotFound(err) {
			return filemodel.MutationResult{}, err
		}
		if err := root.mkdir(request.Path); err != nil {
			return filemodel.MutationResult{}, err
		}
		created, err := root.entry(request.Path)
		if err != nil {
			return filemodel.MutationResult{}, err
		}
		return filemodel.MutationResult{DestinationEntry: &created, AffectedCount: 1, AffectedPathPrefixes: []filemodel.RelativePath{request.Path}}, nil
	})
}

func (s *Store) Write(ctx context.Context, rootPath string, request filemodel.WriteRequest) (filemodel.MutationResult, error) {
	return s.withLock(ctx, rootPath, func(root *root) (filemodel.MutationResult, error) {
		if err := validateText(request.Text, s.config.MaxTextBytes); err != nil {
			return filemodel.MutationResult{}, err
		}
		entry, err := root.entry(request.Path)
		if err != nil {
			return filemodel.MutationResult{}, err
		}
		if entry.Kind != filemodel.EntryKindFile {
			return filemodel.MutationResult{}, filemodel.NewError("type_mismatch", "The path is not a file.")
		}
		if !request.Force {
			if err := requireEntryRevision(entry, request.ExpectedRevision); err != nil {
				return filemodel.MutationResult{}, err
			}
		}
		return root.write(request.Path, request.Text, entry, request.Path.Parent())
	})
}

func (s *Store) Rename(ctx context.Context, rootPath string, request filemodel.RenameRequest) (filemodel.MutationResult, error) {
	return s.withLock(ctx, rootPath, func(root *root) (filemodel.MutationResult, error) {
		source, err := root.entry(request.Path)
		if err != nil {
			return filemodel.MutationResult{}, err
		}
		if err := requireEntryRevision(source, request.ExpectedSourceRevision); err != nil {
			return filemodel.MutationResult{}, err
		}
		if err := root.requireRevision(request.Path.Parent(), request.ExpectedParentRevision); err != nil {
			return filemodel.MutationResult{}, err
		}
		destination := filemodel.RelativePath(request.NewName)
		if request.Path.Parent() != "" {
			destination = request.Path.Parent() + "/" + filemodel.RelativePath(request.NewName)
		}
		if existing, err := root.entry(destination); err == nil {
			if err := requireEntryRevision(existing, request.ExpectedDestinationRevision); err != nil {
				return filemodel.MutationResult{}, err
			}
			return filemodel.MutationResult{}, filemodel.NewError("already_exists", "The destination already exists.")
		} else if !isNotFound(err) {
			return filemodel.MutationResult{}, err
		}
		if err := root.rename(request.Path, destination); err != nil {
			return filemodel.MutationResult{}, err
		}
		moved, err := root.entry(destination)
		if err != nil {
			return filemodel.MutationResult{}, err
		}
		return filemodel.MutationResult{SourceEntry: &source, DestinationEntry: &moved, AffectedCount: 1, AffectedPathPrefixes: []filemodel.RelativePath{request.Path, destination}}, nil
	})
}

func (s *Store) Move(ctx context.Context, rootPath string, request filemodel.MoveRequest) (filemodel.MutationResult, error) {
	return s.withLock(ctx, rootPath, func(root *root) (filemodel.MutationResult, error) {
		if request.DestinationPath.IsWithin(request.SourcePath) && request.SourcePath != request.DestinationPath {
			return filemodel.MutationResult{}, filemodel.NewError("invalid_operation", "A directory cannot be moved into itself.")
		}
		source, err := root.entry(request.SourcePath)
		if err != nil {
			return filemodel.MutationResult{}, err
		}
		if err := requireEntryRevision(source, request.ExpectedSourceRevision); err != nil {
			return filemodel.MutationResult{}, err
		}
		if err := root.requireRevision(request.SourcePath.Parent(), request.ExpectedSourceParentRevision); err != nil {
			return filemodel.MutationResult{}, err
		}
		if err := root.requireRevision(request.DestinationPath.Parent(), request.ExpectedDestinationParentRevision); err != nil {
			return filemodel.MutationResult{}, err
		}
		if existing, err := root.entry(request.DestinationPath); err == nil {
			if !request.Overwrite {
				if err := requireEntryRevision(existing, request.ExpectedDestinationRevision); err != nil {
					return filemodel.MutationResult{}, err
				}
				return filemodel.MutationResult{}, filemodel.NewError("already_exists", "The destination already exists.")
			}
			// overwrite: replace destination under the same lock (VS Code rename overwrite).
			if err := requireEntryRevision(existing, request.ExpectedDestinationRevision); err != nil {
				return filemodel.MutationResult{}, err
			}
			if err := root.remove(request.DestinationPath, true); err != nil {
				return filemodel.MutationResult{}, err
			}
		} else if !isNotFound(err) {
			return filemodel.MutationResult{}, err
		}
		if err := root.rename(request.SourcePath, request.DestinationPath); err != nil {
			return filemodel.MutationResult{}, err
		}
		moved, err := root.entry(request.DestinationPath)
		if err != nil {
			return filemodel.MutationResult{}, err
		}
		return filemodel.MutationResult{SourceEntry: &source, DestinationEntry: &moved, AffectedCount: 1, AffectedPathPrefixes: []filemodel.RelativePath{request.SourcePath, request.DestinationPath}}, nil
	})
}

func (s *Store) Delete(ctx context.Context, rootPath string, request filemodel.DeleteRequest) (filemodel.MutationResult, error) {
	return s.withLock(ctx, rootPath, func(root *root) (filemodel.MutationResult, error) {
		entry, err := root.entry(request.Path)
		if err != nil {
			return filemodel.MutationResult{}, err
		}
		if err := requireEntryRevision(entry, request.ExpectedRevision); err != nil {
			return filemodel.MutationResult{}, err
		}
		if err := root.requireRevision(request.Path.Parent(), request.ExpectedParentRevision); err != nil {
			return filemodel.MutationResult{}, err
		}
		count, err := root.count(request.Path, s.config.MaxRecursiveDeleteEntries)
		if err != nil {
			return filemodel.MutationResult{}, err
		}
		if entry.Kind == filemodel.EntryKindDirectory && count > 1 && !request.Recursive {
			return filemodel.MutationResult{}, filemodel.NewError("directory_not_empty", "The directory is not empty.")
		}
		if err := root.remove(request.Path, request.Recursive); err != nil {
			return filemodel.MutationResult{}, err
		}
		return filemodel.MutationResult{SourceEntry: &entry, AffectedCount: count, AffectedPathPrefixes: []filemodel.RelativePath{request.Path}}, nil
	})
}

// DeleteRegularFile removes one regular workspace file through the rooted store.
// It is intentionally limited to the Git untracked-file workflow, whose status
// preflight is owned by the Git service rather than a browser revision token.
func (s *Store) DeleteRegularFile(ctx context.Context, rootPath string, path filemodel.RelativePath) error {
	_, err := s.withLock(ctx, rootPath, func(root *root) (filemodel.MutationResult, error) {
		entry, err := root.entry(path)
		if err != nil {
			return filemodel.MutationResult{}, err
		}
		if entry.Kind != filemodel.EntryKindFile {
			return filemodel.MutationResult{}, filemodel.NewError("type_mismatch", "Only a regular file can be removed.")
		}
		if err := root.remove(path, false); err != nil {
			return filemodel.MutationResult{}, err
		}
		return filemodel.MutationResult{}, nil
	})
	return err
}

func (s *Store) withLock(ctx context.Context, rootPath string, operation func(*root) (filemodel.MutationResult, error)) (filemodel.MutationResult, error) {
	if err := ctx.Err(); err != nil {
		return filemodel.MutationResult{}, err
	}
	value, _ := s.locks.LoadOrStore(rootPath, &sync.Mutex{})
	lock := value.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	root, err := openRoot(rootPath)
	if err != nil {
		return filemodel.MutationResult{}, err
	}
	defer func() { _ = root.Close() }()
	return operation(root)
}

type root struct {
	path string
	root *os.Root
}

func openRoot(path string) (*root, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return nil, filemodel.NewError("workspace_root_unavailable", "The workspace root is unavailable.")
	}
	info, err := os.Lstat(absolutePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, filemodel.NewError("workspace_root_unavailable", "The workspace root is unavailable.")
		}
		return nil, err
	}
	if !info.IsDir() || unsafeFileMode(info.Mode()) {
		return nil, filemodel.NewError("workspace_root_unavailable", "The workspace root is unavailable.")
	}
	opened, err := os.OpenRoot(absolutePath)
	if err != nil {
		return nil, filemodel.NewError("workspace_root_unavailable", "The workspace root is unavailable.")
	}
	return &root{path: absolutePath, root: opened}, nil
}

func (r *root) Close() error { return r.root.Close() }

func (r *root) resolve(path filemodel.RelativePath) (string, error) {
	if path != "" {
		if _, err := filemodel.ParseRelativePath(path.String(), false); err != nil {
			return "", err
		}
	}
	if err := r.validatePathComponents(path); err != nil {
		return "", err
	}
	candidate := r.path
	if path != "" {
		candidate = filepath.Join(r.path, filepath.FromSlash(path.String()))
	}
	absoluteCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return "", filemodel.InvalidPath("path is invalid")
	}
	relativeCandidate, err := filepath.Rel(r.path, absoluteCandidate)
	if err != nil || relativeCandidate == ".." || strings.HasPrefix(relativeCandidate, ".."+string(filepath.Separator)) || filepath.IsAbs(relativeCandidate) {
		return "", filemodel.InvalidPath("path must remain within the workspace")
	}
	return absoluteCandidate, nil
}

func (r *root) validatePathComponents(path filemodel.RelativePath) error {
	if path == "" {
		return nil
	}
	segments := strings.Split(path.String(), "/")
	current := ""
	for index, segment := range segments {
		if current == "" {
			current = segment
		} else {
			current += "/" + segment
		}
		info, err := r.root.Lstat(current)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return fileError(err)
		}
		if unsafeFileMode(info.Mode()) {
			return filemodel.NewError("symlink_unsupported", "Links and special filesystem entries are unsupported.")
		}
		if index < len(segments)-1 && !info.IsDir() {
			return filemodel.NewError("type_mismatch", "An intermediate workspace entry is not a directory.")
		}
	}
	return nil
}

func (r *root) entry(path filemodel.RelativePath) (filemodel.Entry, error) {
	if _, err := r.resolve(path); err != nil {
		return filemodel.Entry{}, err
	}
	name := path.String()
	if name == "" {
		name = "."
	}
	info, err := r.root.Lstat(name)
	if err != nil {
		return filemodel.Entry{}, fileError(err)
	}
	if unsafeFileMode(info.Mode()) {
		return filemodel.Entry{}, filemodel.NewError("symlink_unsupported", "Links and special filesystem entries are unsupported.")
	}
	kind := filemodel.EntryKindFile
	if info.IsDir() {
		kind = filemodel.EntryKindDirectory
	} else if !info.Mode().IsRegular() {
		return filemodel.Entry{}, filemodel.NewError("type_mismatch", "The filesystem entry is unsupported.")
	}
	revisionValue, err := r.revision(path, info)
	if err != nil {
		return filemodel.Entry{}, err
	}
	entry := filemodel.Entry{Path: path, Name: path.Base(), Kind: kind, Size: info.Size(), ModifiedAt: info.ModTime(), Revision: revisionValue}
	if path == "" {
		entry.Name = ""
	}
	return entry, nil
}

type listedEntries struct {
	items     []filemodel.Entry
	truncated bool
}

func (r *root) list(path filemodel.RelativePath, limit int) (listedEntries, error) {
	if _, err := r.resolve(path); err != nil {
		return listedEntries{}, err
	}
	name := path.String()
	if name == "" {
		name = "."
	}
	directory, err := r.root.Open(name)
	if err != nil {
		return listedEntries{}, fileError(err)
	}
	defer func() { _ = directory.Close() }()
	items, err := directory.ReadDir(limit + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return listedEntries{}, err
	}
	result := listedEntries{truncated: len(items) > limit}
	if result.truncated {
		items = items[:limit]
	}
	for _, item := range items {
		child := filemodel.RelativePath(item.Name())
		if path != "" {
			child = path + "/" + child
		}
		entry, err := r.entry(child)
		if err != nil {
			return listedEntries{}, err
		}
		result.items = append(result.items, entry)
	}
	sort.SliceStable(result.items, func(i, j int) bool {
		if result.items[i].Kind != result.items[j].Kind {
			return result.items[i].Kind == filemodel.EntryKindDirectory
		}
		return result.items[i].Name < result.items[j].Name
	})
	return result, nil
}

func (r *root) requireRevision(path filemodel.RelativePath, expected string) error {
	entry, err := r.entry(path)
	if err != nil {
		return err
	}
	return requireEntryRevision(entry, expected)
}

func requireEntryRevision(entry filemodel.Entry, expected string) error {
	// Empty token means the caller did not request optimistic concurrency.
	if expected == "" {
		return nil
	}
	if entry.Revision != expected {
		return filemodel.Conflict(&entry)
	}
	return nil
}

func (r *root) create(path filemodel.RelativePath, text string) error {
	if _, err := r.resolve(path); err != nil {
		return err
	}
	file, err := r.root.OpenFile(path.String(), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fileError(err)
	}
	defer func() { _ = file.Close() }()
	if _, err := io.WriteString(file, text); err != nil {
		return err
	}
	return file.Sync()
}

func (r *root) mkdir(path filemodel.RelativePath) error {
	if _, err := r.resolve(path); err != nil {
		return err
	}
	if err := r.root.Mkdir(path.String(), 0o700); err != nil {
		return fileError(err)
	}
	return nil
}

func (r *root) write(path filemodel.RelativePath, text string, source filemodel.Entry, parent filemodel.RelativePath) (filemodel.MutationResult, error) {
	if _, err := r.resolve(path); err != nil {
		return filemodel.MutationResult{}, err
	}
	temporary := filemodel.RelativePath(fmt.Sprintf("%s.termbridge-%d.tmp", path, time.Now().UnixNano()))
	file, err := r.root.OpenFile(temporary.String(), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return filemodel.MutationResult{}, fileError(err)
	}
	defer func() { _ = r.root.Remove(temporary.String()) }()
	if _, err := io.WriteString(file, text); err != nil {
		_ = file.Close()
		return filemodel.MutationResult{}, err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return filemodel.MutationResult{}, err
	}
	if err := file.Close(); err != nil {
		return filemodel.MutationResult{}, err
	}
	current, err := r.entry(path)
	if err != nil {
		return filemodel.MutationResult{}, err
	}
	if current.Revision != source.Revision {
		return filemodel.MutationResult{}, filemodel.Conflict(&current)
	}
	if err := r.root.Rename(temporary.String(), path.String()); err != nil {
		return filemodel.MutationResult{}, fileError(err)
	}
	updated, err := r.entry(path)
	if err != nil {
		return filemodel.MutationResult{}, err
	}
	return filemodel.MutationResult{SourceEntry: &source, DestinationEntry: &updated, AffectedCount: 1, AffectedPathPrefixes: []filemodel.RelativePath{parent}}, nil
}

func (r *root) rename(source filemodel.RelativePath, destination filemodel.RelativePath) error {
	if _, err := r.resolve(source); err != nil {
		return err
	}
	if _, err := r.resolve(destination); err != nil {
		return err
	}
	if err := r.root.Rename(source.String(), destination.String()); err != nil {
		return fileError(err)
	}
	return nil
}

func (r *root) count(path filemodel.RelativePath, limit int) (int, error) {
	count := 0
	var walk func(filemodel.RelativePath) error
	walk = func(current filemodel.RelativePath) error {
		count++
		if count > limit {
			return filemodel.NewError("file_too_large", "The recursive operation exceeds its entry limit.")
		}
		entry, err := r.entry(current)
		if err != nil {
			return err
		}
		if entry.Kind != filemodel.EntryKindDirectory {
			return nil
		}
		items, err := r.list(current, limit)
		if err != nil {
			return err
		}
		for _, item := range items.items {
			if err := walk(item.Path); err != nil {
				return err
			}
		}
		if items.truncated {
			return filemodel.NewError("file_too_large", "The recursive operation exceeds its entry limit.")
		}
		return nil
	}
	if err := walk(path); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *root) remove(path filemodel.RelativePath, recursive bool) error {
	if _, err := r.resolve(path); err != nil {
		return err
	}
	entry, err := r.entry(path)
	if err != nil {
		return err
	}
	if entry.Kind == filemodel.EntryKindDirectory && recursive {
		items, err := r.list(path, int(^uint(0)>>1))
		if err != nil {
			return err
		}
		for _, item := range items.items {
			if err := r.remove(item.Path, true); err != nil {
				return err
			}
		}
	}
	if err := r.root.Remove(path.String()); err != nil {
		return fileError(err)
	}
	return nil
}

func (r *root) revision(path filemodel.RelativePath, info fs.FileInfo) (string, error) {
	digest := sha256.New()
	_, _ = fmt.Fprintf(digest, "%s\x00%s\x00%d\x00%d\x00%d", path, info.Mode().String(), info.Size(), info.ModTime().UnixNano(), info.ModTime().Unix())
	if info.Mode().IsRegular() {
		file, err := r.root.Open(path.String())
		if err != nil {
			return "", fileError(err)
		}
		defer func() { _ = file.Close() }()
		if _, err := io.Copy(digest, file); err != nil {
			return "", err
		}
	}
	return base64.RawURLEncoding.EncodeToString(digest.Sum(nil)), nil
}

// readBounded returns raw file bytes up to limit.
// Read is byte-transparent (VS Code FileSystemProvider / media preview need PNG etc.).
// Write/create still enforce UTF-8 text via validateText.
func readBounded(root *root, path filemodel.RelativePath, limit int64) ([]byte, error) {
	if _, err := root.resolve(path); err != nil {
		return nil, err
	}
	file, err := root.root.Open(path.String())
	if err != nil {
		return nil, fileError(err)
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, filemodel.NewError("file_too_large", "The file exceeds the size limit.")
	}
	return data, nil
}

func validateText(text string, limit int64) error {
	if int64(len(text)) > limit {
		return filemodel.NewError("file_too_large", "The text exceeds the file limit.")
	}
	if !utf8.ValidString(text) || strings.IndexByte(text, 0) >= 0 {
		return filemodel.NewError("file_not_text", "Only UTF-8 text is supported.")
	}
	return nil
}

func isNotFound(err error) bool {
	return errors.Is(err, os.ErrNotExist) || filemodel.CodeOf(err) == "file_not_found"
}

func unsafeFileMode(mode fs.FileMode) bool {
	return mode&os.ModeSymlink != 0 || mode&os.ModeIrregular != 0 || mode&os.ModeDevice != 0 || mode&os.ModeNamedPipe != 0 || mode&os.ModeSocket != 0
}

func fileError(err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return filemodel.NewError("file_not_found", "The workspace entry was not found.")
	}
	if errors.Is(err, fs.ErrExist) {
		return filemodel.NewError("already_exists", "The destination already exists.")
	}
	return err
}

var _ filemodel.Store = (*Store)(nil)
