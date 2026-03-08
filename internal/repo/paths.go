package repo

const (
	// ObjectsDir is the subdirectory for storing all content-addressable objects.
	ObjectsDir = ".kitcat/objects"

	// RefsDir is the subdirectory for storing references like heads and tags.
	RefsDir = ".kitcat/refs"

	// HeadsDir is the subdirectory for storing branch heads.
	HeadsDir = ".kitcat/refs/heads"

	// TagsDir is the subdirectory for storing tags.
	TagsDir = ".kitcat/refs/tags"

	// IndexPath is the full path to the index file.
	IndexPath = ".kitcat/index"

	// HeadPath is the full path to the HEAD file.
	HeadPath = ".kitcat/HEAD"

	// RepoDir is the full path to the repository directory.
	Dir = ".kitcat"
)
