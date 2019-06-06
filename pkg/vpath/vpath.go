package vpath

import (
	"github.com/zeebo/errs"
	"storj.io/storj/pkg/storj"
)

// The searcher allows one to find the matching most encrypted key and path for
// some unencrypted path. It also reports a mapping of encrypted to unencrypted paths
// at the searched for unencrypted path.
//
// For example, if the searcher contains the mappings
//
//    u1/u2/u3    => <e1/e2/e3, k3>
//    u1/u2/u3/u4 => <e1/e2/e3/e4, k4>
//    u1/u5       => <e1/e5, k5>
//    u6          => <e6, k6>
//    u6/u7/u8    => <e6/e7/e8, k8>
//
// Then the following lookups have outputs
//
//    u1          => <{e2:u2, e5:u5}, u1, nil>
//    u1/u2/u3    => <{e4:u4}, u1/u2/u3, <e1/e2/e3, k3>>
//    u1/u2/u3/u6 => <{}, u1/u2/u3/, <e1/e2/e3, k3>>
//    u1/u2/u3/u4 => <{}, u1/u2/u3/u4, <e1/e2/e3/e4, k4>>
//    u6/u7       => <{e8:u8}, u6/, <e6, k6>>
type Searcher struct {
	root *node
}

// node is a node in the searcher graph. It may contain an encryption key and encrypted path,
// a list of children nodes, and data to ensure a bijection between encrypted and unencrypted
// path entries.
type node struct {
	children    map[string]*node  // unenc => node
	revealed    map[string]string // enc => unenc
	invRevealed map[string]string // unenc => enc
	base        *Base
}

// Base represents a key with which to derive further keys at some encrypted path.
type Base struct {
	Encrypted storj.Path
	Key       storj.Key
}

// NewSearcher constructs a Searcher.
func NewSearcher() *Searcher {
	return &Searcher{
		root: newNode(),
	}
}

// newNode constructs a node.
func newNode() *node {
	return &node{
		children:    make(map[string]*node),
		revealed:    make(map[string]string),
		invRevealed: make(map[string]string),
	}
}

// Add creates a mapping from the unencrypted path to the encrypted path and key.
func (s *Searcher) Add(unencrypted, encrypted storj.Path, key storj.Key) error {
	return s.root.add(newPathWalker(unencrypted), newPathWalker(encrypted), &Base{
		Encrypted: encrypted,
		Key:       key,
	})
}

// add places the paths and base into the node tree structure.
func (n *node) add(unenc, enc pathWalker, base *Base) error {
	if unenc.Empty() != enc.Empty() {
		return errs.New("encrypted and unencrypted paths had different number of components")
	}

	// If we're done walking the paths, this node must have the provided base.
	if unenc.Empty() {
		n.base = base
		return nil
	}

	// Walk to the next parts and ensure they're consistent with previous additions.
	unencPart, encPart := unenc.Next(), enc.Next()
	if revealedPart, ok := n.revealed[encPart]; ok && revealedPart != unencPart {
		return errs.New("conflicting encrypted parts for unencrypted path")
	}
	if invRevealedPart, ok := n.invRevealed[unencPart]; ok && invRevealedPart != encPart {
		return errs.New("conflicting encrypted parts for unencrypted path")
	}

	// Look up the child in the tree, allocating if necessary.
	child, ok := n.children[unencPart]
	if !ok {
		child = newNode()
		n.children[unencPart] = child
		n.revealed[encPart] = unencPart
		n.invRevealed[unencPart] = encPart
	}

	// Recurse to the next node in the tree.
	return child.add(unenc, enc, base)
}

// Lookup finds the matching most unencrypted path added to the Searcher, reports how much
// of the path matched, any known unencrypted paths at the requested path, and if a key
// and encrypted path exists for the unencrypted path.
func (s *Searcher) Lookup(unencrypted string) (
	revealed map[string]string, consumed string, base *Base) {

	return s.root.lookup(newPathWalker(unencrypted), "", nil)
}

// lookup searches for the path in the node tree structure.
func (n *node) lookup(path pathWalker, bestConsumed string, bestBase *Base) (
	map[string]string, string, *Base) {

	// Keep track of the best match so far.
	if n.base != nil || bestBase == nil {
		bestConsumed, bestBase = path.Consumed(), n.base
	}

	// If we're done walking the path, then return our best match along with the
	// revealed paths at this node.
	if path.Empty() {
		return n.revealed, bestConsumed, bestBase
	}

	// Walk to the next node in the tree. If there is no node, then report our best
	// match.
	child, ok := n.children[path.Next()]
	if !ok {
		return nil, bestConsumed, bestBase
	}

	// Recurse to the next node in the tree.
	return child.lookup(path, bestConsumed, bestBase)
}
