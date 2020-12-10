package commitgraph

import (
	"sort"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/gitserver"
)

type Graph struct {
	commitGraphView   *CommitGraphView
	graph             map[string][]string
	reverseGraph      map[string][]string
	commits           []string
	ancestorUploads   map[string]map[string]UploadMeta
	descendantUploads map[string]map[string]UploadMeta
}

type Envelope struct {
	Uploads *VisibilityRelationship
	Links   *LinkRelationship
}

type VisibilityRelationship struct {
	Commit  string
	Uploads []UploadMeta
}

type LinkRelationship struct {
	Commit             string
	Ancestor           *string
	AncestorDistance   uint32
	Descendant         *string
	DescendantDistance uint32
}

// NewGraph creates a commit graph decorated with the set of uploads visible from that commit
// based on the given commit graph and complete set of LSIF upload metadata.
func NewGraph(commitGraph *gitserver.CommitGraph, commitGraphView *CommitGraphView) *Graph {
	graph := commitGraph.Graph()
	reverseGraph := reverseGraph(graph)
	order := commitGraph.Order()

	ancestorUploads := populateUploadsByTraversal(graph, reverseGraph, order, commitGraphView, false)
	descendantUploads := populateUploadsByTraversal(reverseGraph, graph, order, commitGraphView, true)
	sort.Strings(order)

	return &Graph{
		commitGraphView:   commitGraphView,
		graph:             graph,
		reverseGraph:      reverseGraph,
		commits:           order,
		ancestorUploads:   ancestorUploads,
		descendantUploads: descendantUploads,
	}
}

// UploadsVisibleAtCommit returns the set of uploads that are visible from the given commit.
func (g *Graph) UploadsVisibleAtCommit(commit string) []UploadMeta {
	ancestorUploads, ancestorDistance := traverseForUploads(g.graph, g.ancestorUploads, commit)
	descendantUploads, descendantDistance := traverseForUploads(g.reverseGraph, g.descendantUploads, commit)
	return combineVisibleUploadsForCommit(ancestorUploads, descendantUploads, ancestorDistance, descendantDistance)
}

// Stream returns a channel of envelope values which indicate either the set of visible uploads
// at a particular commit, or the nearest neighbors at a particular commit, depending on the
// value within the envelope.
func (g *Graph) Stream() <-chan Envelope {
	ch := make(chan Envelope)

	go func() {
		defer close(ch)

		for _, commit := range g.commits {
			ancestorCommit, ancestorDistance, found1 := traverseForCommit(g.graph, g.ancestorUploads, commit)
			descendantCommit, descendantDistance, found2 := traverseForCommit(g.reverseGraph, g.descendantUploads, commit)
			if !found1 && !found2 {
				continue
			}

			ancestorVisibleUploads := g.ancestorUploads[ancestorCommit]
			descendantVisibleUploads := g.descendantUploads[descendantCommit]
			if len(ancestorVisibleUploads)+len(descendantVisibleUploads) == 0 {
				continue
			}

			uploads := combineVisibleUploadsForCommit(
				ancestorVisibleUploads,
				descendantVisibleUploads,
				ancestorDistance,
				descendantDistance,
			)

			threshold := 1
			if found1 && found2 {
				threshold = 2
			}

			if (found1 && ancestorDistance == 0) || (found2 && descendantDistance == 0) || len(uploads) <= threshold {
				ch <- Envelope{
					Uploads: &VisibilityRelationship{
						Commit:  commit,
						Uploads: uploads,
					},
				}
			} else {
				// Otherwise, we have more than a pair of uploads. Because we also have a
				// very cheap way of reconstructing this particular commit's visible uploads
				// from its ancestors and descendants, we store that relationships which is
				// much smaller when the number of distinct LSIF roots becomes large.

				ch <- Envelope{
					Links: &LinkRelationship{
						Commit:             commit,
						Ancestor:           strPtrOk(ancestorCommit, found1),
						AncestorDistance:   ancestorDistance,
						Descendant:         strPtrOk(descendantCommit, found2),
						DescendantDistance: descendantDistance,
					},
				}
			}
		}
	}()

	return ch
}

func strPtrOk(value string, ok bool) *string {
	if !ok {
		return nil
	}

	return &value
}

// Gather reads the graph's stream to completion and returns a map of the values. This
// method is only used for convenience and testing and should not be used in a hot path.
// It can be VERY memory intensive in production to have a reference to each commit's
// upload metadata concurrently.
func (g *Graph) Gather() (uploads map[string][]UploadMeta, links map[string]LinkRelationship) {
	uploads = map[string][]UploadMeta{}
	links = map[string]LinkRelationship{}

	for v := range g.Stream() {
		if v.Uploads != nil {
			uploads[v.Uploads.Commit] = v.Uploads.Uploads
		}
		if v.Links != nil {
			links[v.Links.Commit] = *v.Links
		}
	}

	return uploads, links
}

// reverseGraph returns the reverse of the given graph by flipping all the edges.
func reverseGraph(graph map[string][]string) map[string][]string {
	reverse := make(map[string][]string, len(graph))
	for child := range graph {
		reverse[child] = nil
	}

	for child, parents := range graph {
		for _, parent := range parents {
			reverse[parent] = append(reverse[parent], child)
		}
	}

	return reverse
}

// populateUploadsByTraversal populates a map from select commits (see below) to another map from
// tokens to upload meta value. Select commits are any commits that satisfy one of the following
// properties:
//
//   1. They define an upload
//   2. They have multiple children
//   3. They have multiple parents
//   4. Their parent has multiple children
//   5. Their child has multiple parents
//
// For all remaining commits, we can easily re-calculate the visible uploads without storing them.
// All such commits have a parent whose only child is the commit (or has no parent), and a single
// child whose only parent is the commit (or has no children). This means that there is a single
// unambiguous path to an ancestor with calculated data, and symmetrically in the other direction.
// See combineVisibleUploadsForCommit for additional details.
func populateUploadsByTraversal(graph, reverseGraph map[string][]string, order []string, commitGraphView *CommitGraphView, reverse bool) map[string]map[string]UploadMeta {
	uploads := make(map[string]map[string]UploadMeta, len(order))
	for i, commit := range order {
		if reverse {
			commit = order[len(order)-i-1]
		}

		parents := graph[commit]
		children := reverseGraph[commit]

		_, ok := commitGraphView.Meta[commit]
		if !ok && // ¬Property 1
			len(children) <= 1 && // ¬Property 2
			len(parents) <= 1 && // ¬Property 3
			(len(parents) == 0 || len(reverseGraph[parents[0]]) == 1) && // ¬Property 4
			(len(children) == 0 || len(graph[children[0]]) == 1) { // ¬Property 5
			continue
		}

		ancestors := parents
		distance := uint32(1)

		// Find nearest ancestors with data. If we end the loop with multiple ancestors, we know
		// that they are all the same distance from the starting commit, and all of them have
		// data as they've already been processed and all satisfy Property 5 above.
		for len(ancestors) == 1 {
			if _, ok := uploads[ancestors[0]]; ok {
				break
			}

			distance++
			ancestors = graph[ancestors[0]]
		}

		uploads[commit] = populateUploadsForCommit(uploads, ancestors, distance, commitGraphView, commit)
	}

	return uploads
}

// populateUploadsForCommit populates the items stored in the given mapping for the given commit.
// The uploads considered visible for a commit include:
//
//   1. the set of uploads defined on that commit, and
//   2. the set of  uploads visible from the ancestors (or descendants) with the minimum distance
//      for equivalent root and indexer values.
//
// If two ancestors have different uploads visible for the same root and indexer, the one with the
// smaller distance to the source commit will shadow the other. Similarly, If an ancestor and the
// child commit define uploads for the same root and indexer pair, the upload defined on the commit
// will shadow the upload defined on the ancestor.
func populateUploadsForCommit(uploads map[string]map[string]UploadMeta, ancestors []string, distance uint32, commitGraphView *CommitGraphView, commit string) map[string]UploadMeta {
	// The capacity chosen here is an underestimate, but seems to perform well in
	// benchmarks using live user data. We have attempted to make this value more
	// precise to minimize the number of re-hash operations, but any counting we
	// do requires auxiliary space and takes additional CPU to traverse the graph.
	capacity := len(commitGraphView.Meta[commit])
	for _, ancestor := range ancestors {
		if temp := len(uploads[ancestor]); temp > capacity {
			capacity = temp
		}
	}
	uploadsByToken := make(map[string]UploadMeta, capacity)

	// Populate uploads defined here
	for _, upload := range commitGraphView.Meta[commit] {
		token := commitGraphView.Tokens[upload.UploadID]
		uploadsByToken[token] = upload
	}

	// Combine with uploads visible from the nearest ancestors
	for _, ancestor := range ancestors {
		for _, upload := range uploads[ancestor] {
			token := commitGraphView.Tokens[upload.UploadID]

			// Increase distance from source before comparison
			upload.Flags += distance

			// Only update upload for this token if distance of new upload is less than current one
			if currentUpload, ok := uploadsByToken[token]; !ok || replaces(upload, currentUpload) {
				uploadsByToken[token] = upload
			}
		}
	}

	return uploadsByToken
}

// traverseForUploads returns the value in the given uploads map whose key matches the first ancestor
// in the graph with a value present in the map. The distance in the graph between the original commit
// and the ancestor is also returned.
func traverseForUploads(graph map[string][]string, uploads map[string]map[string]UploadMeta, commit string) (map[string]UploadMeta, uint32) {
	commit, distance, _ := traverseForCommit(graph, uploads, commit)
	return uploads[commit], distance
}

// traverseForCommit returns the commit in the given uploads map matching the first ancestor in
// the graph with a value present in the map. The distance in the graph between the original commit
// and the ancestor is also returned.
//
// NOTE: We assume that each commit with multiple parents have been assigned data while walking
// the graph in topological order. If that is not the case, one parent will be chosen arbitrarily.
func traverseForCommit(graph map[string][]string, uploads map[string]map[string]UploadMeta, commit string) (string, uint32, bool) {
	for distance := uint32(0); ; distance++ {
		if _, ok := uploads[commit]; ok {
			return commit, distance, true
		}

		parents := graph[commit]
		if len(parents) == 0 {
			return "", 0, false
		}

		commit = parents[0]
	}
}

// combineVisibleUploadsForCommit combines sets of uploads visible by looking in opposite directions
// in the graph. This will produce a flat list of upload meta objects for each commit that consists of:
//
//   1. the set of ancestor-visible uploads,
//   2. the set of descendant-visible uploads where there does not exist an ancestor-visible upload
//      with an equivalent root an indexer value, and
//   3. the set of descendant-visible uploads where there exists an ancestor-visible upload with an
//      equivalent root and indexer value but a greater distance. In this case, the ancestor-visible
//      upload is also present in the list, but is flagged as overwritten.
func combineVisibleUploadsForCommit(ancestorVisibleUploads, descendantVisibleUploads map[string]UploadMeta, ancestorDistance, descendantDistance uint32) []UploadMeta {
	// The capacity chosen here is an underestimate, but seems to perform well in
	// benchmarks using live user data. See populateUploadsForCommit for a simlar
	// implementation note.
	capacity := len(ancestorVisibleUploads)
	if temp := len(descendantVisibleUploads); temp > capacity {
		capacity = temp
	}
	uploads := make([]UploadMeta, 0, capacity)

	for token, ancestorUpload := range ancestorVisibleUploads {
		ancestorUpload.Flags += ancestorDistance
		ancestorUpload.Flags |= FlagAncestorVisible

		if descendantUpload, ok := descendantVisibleUploads[token]; ok {
			descendantUpload.Flags += descendantDistance
			descendantUpload.Flags &^= FlagAncestorVisible

			// If the matching descendant upload has a smaller distance
			if replaces(descendantUpload, ancestorUpload) {
				// Mark the ancestor upload as overwritten
				ancestorUpload.Flags |= FlagOverwritten
				// Add the descendant upload
				uploads = append(uploads, descendantUpload)
			}
		}

		// Add all ancestor uploads
		uploads = append(uploads, ancestorUpload)
	}

	for token, descendantUpload := range descendantVisibleUploads {
		descendantUpload.Flags += descendantDistance
		descendantUpload.Flags &^= FlagAncestorVisible

		// Add all descendant uploads that have no ancestor matches
		if _, ok := ancestorVisibleUploads[token]; !ok {
			uploads = append(uploads, descendantUpload)
		}
	}

	return uploads
}

// replaces returns true if upload1 has a smaller distance than upload2.
// Ties are broken by the minimum upload identifier to remain determinstic.
func replaces(upload1, upload2 UploadMeta) bool {
	distance1 := upload1.Flags & MaxDistance
	distance2 := upload2.Flags & MaxDistance

	return distance1 < distance2 || (distance1 == distance2 && upload1.UploadID < upload2.UploadID)
}
