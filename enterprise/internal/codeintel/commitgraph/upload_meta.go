package commitgraph

// UploadMeta represents the visibility of an LSIF upload from a particular location
// on a repository's commit graph. The Flags field describes the visibility of the
// upload from the current viewer's perspective.
type UploadMeta struct {
	UploadID int
	Distance uint32
}
