// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package asset_test

import (
	"io/ioutil"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/cmd/internal/asset"
)

func TestAssets(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	require.NoError(t, ioutil.WriteFile(ctx.File("a", "example.css"), []byte("/a/example.css"), 0644))
	require.NoError(t, ioutil.WriteFile(ctx.File("a", "example.js"), []byte("/a/example.js"), 0644))
	require.NoError(t, ioutil.WriteFile(ctx.File("alpha.css"), []byte("/alpha.css"), 0644))
	require.NoError(t, ioutil.WriteFile(ctx.File("x", "beta.css"), []byte("/x/beta.css"), 0644))
	require.NoError(t, ioutil.WriteFile(ctx.File("x", "y", "gamma.js"), []byte("/x/y/gamma.js"), 0644))

	root, err := asset.ReadDir(ctx.Dir())
	require.NotNil(t, root)
	require.NoError(t, err)

	// sparse check on the content
	require.Equal(t, root.Name, "")
	require.Equal(t, len(root.Children), 3)

	require.Equal(t, root.Children[0].Name, "a")

	require.Equal(t, root.Children[1].Name, "alpha.css")
	require.Equal(t, root.Children[1].Data, []byte("/alpha.css"))

	require.Equal(t, root.Children[2].Name, "x")
	require.Equal(t, root.Children[2].Children[1].Children[0].Name, "gamma.js")

	var walk func(prefix string, node *asset.Asset)
	walk = func(prefix string, node *asset.Asset) {
		if !node.Mode.IsDir() {
			assert.Equal(t, string(node.Data), path.Join(prefix, node.Name))
		} else {
			assert.Equal(t, string(node.Data), "")
		}

		for _, child := range node.Children {
			walk(path.Join(prefix, node.Name), child)
		}
	}
	walk("/", root)

	inmemory := asset.Inmemory(root)
	for path, node := range inmemory.Index {
		if !node.Mode.IsDir() {
			assert.Equal(t, string(node.Data), path)
		} else {
			assert.Equal(t, string(node.Data), "")
		}
	}
}
