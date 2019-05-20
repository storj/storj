package vpath

import "github.com/zeebo/errs"

// The searcher ...
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

type node struct {
	children    map[string]*node  // unenc => node
	revealed    map[string]string // enc => unenc
	invRevealed map[string]string // unenc => enc
	base        *Base
}

type Base struct {
	Encrypted string
	Key       []byte
}

func NewSearcher() *Searcher {
	return &Searcher{
		root: newNode(),
	}
}

func newNode() *node {
	return &node{
		children:    make(map[string]*node),
		revealed:    make(map[string]string),
		invRevealed: make(map[string]string),
	}
}

func (s *Searcher) Add(unencrypted, encrypted string, key []byte) error {
	return s.root.add(newPathWalker(unencrypted), newPathWalker(encrypted), &Base{
		Encrypted: encrypted,
		Key:       key,
	})
}

func (n *node) add(unenc, enc pathWalker, base *Base) error {
	if unenc.Empty() != enc.Empty() {
		return errs.New("encrypted and unencrypted paths had different number of components")
	}

	if unenc.Empty() {
		n.base = base
		return nil
	}

	unencPart, encPart := unenc.Next(), enc.Next()
	if revealedPart, ok := n.revealed[encPart]; ok && revealedPart != unencPart {
		return errs.New("conflicting encrypted parts for unencrypted path")
	}
	if invRevealedPart, ok := n.invRevealed[unencPart]; ok && invRevealedPart != encPart {
		return errs.New("conflicting encrypted parts for unencrypted path")
	}

	child, ok := n.children[unencPart]
	if !ok {
		child = newNode()
		n.children[unencPart] = child
		n.revealed[encPart] = unencPart
		n.invRevealed[unencPart] = encPart
	}

	return child.add(unenc, enc, base)
}

func (s *Searcher) Lookup(unencrypted string) (
	revealed map[string]string, consumed string, base *Base) {

	return s.root.lookup(newPathWalker(unencrypted), "", nil)
}

func (n *node) lookup(path pathWalker, bestConsumed string, bestBase *Base) (
	map[string]string, string, *Base) {

	if n.base != nil || bestBase == nil {
		bestConsumed, bestBase = path.Consumed(), n.base
	}

	if path.Empty() {
		return n.revealed, bestConsumed, bestBase
	}

	child, ok := n.children[path.Next()]
	if !ok {
		return nil, bestConsumed, bestBase
	}

	return child.lookup(path, bestConsumed, bestBase)
}
