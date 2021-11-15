// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package multinode

import (
	"testing"

	"github.com/go-rod/rod"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/testsuite/ui/uitest"
)

func TestAddFirstNodeEmptyInputs(t *testing.T) {
	uitest.Multinode(t, 1, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser) {
		startPage := planet.Multinodes[0].ConsoleURL() + "/add-first-node"
		page := openPage(browser, startPage)

		addNodeButton := page.MustElement(".add-first-node__left-area__button")
		require.Equal(t, "Add Node", addNodeButton.MustText())
		addNodeButton.MustClick()

		nodeIDError := page.MustElementX("(//*[@class=\"label-container__main__error\"])[1]").MustText()
		require.Equal(t, "This field is required. Please enter a valid node ID", nodeIDError)

		ipAddressError := page.MustElementX("(//*[@class=\"label-container__main__error\"])[2]").MustText()
		require.Equal(t, "This field is required. Please enter a valid node Public Address", ipAddressError)

		apiKeyError := page.MustElementX("(//*[@class=\"label-container__main__error\"])[3]").MustText()
		require.Equal(t, "This field is required. Please enter a valid API Key", apiKeyError)
	})
}

func TestAddFirstNodeSuccessful(t *testing.T) {
	uitest.Multinode(t, 1, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser) {
		startPage := planet.Multinodes[0].ConsoleURL() + "/add-first-node"
		page := openPage(browser, startPage)

		node := planet.StorageNodes[0]

		page.MustElement("input#Node\\ ID.headered-input").MustInput(node.ID().String())
		page.MustElement("input#Public\\ IP\\ Address.headered-input").MustInput(node.Addr())
		page.MustElement("input#API\\ Key.headered-input").MustInput(node.APIKey())

		addNodeButton := page.MustElement(".add-first-node__left-area__button")
		require.Equal(t, "Add Node", addNodeButton.MustText())
		addNodeButton.MustClick()
		page.MustWaitNavigation()

		nodesPageTitle := page.MustElement("h1.my-nodes__title").MustText()
		require.Equal(t, "My Nodes", nodesPageTitle)
	})
}
