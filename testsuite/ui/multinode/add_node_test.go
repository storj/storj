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

func TestAddNewNodeButton(t *testing.T) {
	uitest.Multinode(t, 1, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser) {
		startPage := planet.Multinodes[0].ConsoleURL() + "/add-first-node"
		page := openPage(browser, startPage)

		node := planet.StorageNodes[0]
		node2 := planet.StorageNodes[1]

		page.MustElement("input#Node\\ ID.headered-input").MustInput(node.ID().String())
		page.MustElement("input#Public\\ IP\\ Address.headered-input").MustInput(node.Addr())
		page.MustElement("input#API\\ Key.headered-input").MustInput(node.APIKey())

		addNodeButton := page.MustElement(".add-first-node__left-area__button")
		require.Equal(t, "Add Node", addNodeButton.MustText())
		addNodeButton.MustClick()
		page.MustWaitNavigation()

		nodesPageTitle := page.MustElement("h1.my-nodes__title").MustText()
		require.Equal(t, "My Nodes", nodesPageTitle)

		// Test- Clicks on new node button and checks if it opens up add new node modal
		newNodeButton := page.MustElementX("//span[contains(text(),'New Node')]")
		require.Equal(t, "New Node", newNodeButton.MustText())
		newNodeButton.MustClick()
		addNewNodeTitle := page.MustElement("h2:nth-child(1)").MustText()
		require.Equal(t, "Add New Node", addNewNodeTitle)

		// Test- Cancels the addition of new node in dashboard by clicking X button to cancel
		xCancelButton := page.MustElement("div.modal__cross > svg:nth-child(1)")
		xCancelButton.MustClick()
		require.Equal(t, "My Nodes", nodesPageTitle)

		// Test- Cancels the addition of new node in dashboard by clicking on cancel button
		newNodeButton.MustClick()
		require.Equal(t, "Add New Node", addNewNodeTitle)
		CancelButton := page.MustElement("div.container.white:nth-child(1)")
		CancelButton.MustClick()
		require.Equal(t, "My Nodes", nodesPageTitle)

		// Repeat Test- Change Node Display Name
		moreOptionsButton := page.MustElement("div.options-button > svg:nth-child(1)")
		moreOptionsButton.MustClick()
		updateName := page.MustElementX("//div[contains(text(),'Update Name')]")
		updateName.MustClick()
		setNewNameTitle := page.MustElement("div.modal__header > h2:nth-child(1)").MustText()
		// checks for new name modal and checks if name was changed
		require.Equal(t, "Set name for node", setNewNameTitle)
		newDisplayNameField := page.MustElementX("//input[@id='Displayed name']")
		newDisplayNameField.MustInput("newnodename")
		setNameButton := page.MustElement("div.container:nth-child(2)")
		setNameButton.MustClick()
		originalNodeID := page.MustElement("tr.table-item:nth-child(1) > th.align-left:nth-child(1)").MustText()
		require.Equal(t, node.ID().String(), originalNodeID)
		newNodeDisplay := page.MustElementX("//th[contains(text(),'newnodename')]").MustText()
		require.Equal(t, "newnodename", newNodeDisplay)

		// Test- Add node that already exists, node doesn't get added and after trying to add it it goes back to My Nodes Page
		newNodeButton.MustClick()
		enterNodeIDfield := page.MustElementX("//input[@id='Node ID']")
		enterNodeIDfield.MustInput(node.ID().String())
		enterPublicIPAddress := page.MustElementX("//input[@id='Public IP Address']")
		enterPublicIPAddress.MustInput(node.Addr())
		enterAPIKey := page.MustElementX("//input[@id='API Key']")
		enterAPIKey.MustInput(node.APIKey())
		CreateButton := page.MustElement("div.container:nth-child(2)")
		CreateButton.MustClick()
		require.Equal(t, "My Nodes", nodesPageTitle)

		// Test- Adding a node with new node button
		newNodeButton.MustClick()
		enterNodeIDfield1 := page.MustElementX("//input[@id='Node ID']")
		enterNodeIDfield1.MustInput(node2.ID().String())
		enterPublicIPAddress1 := page.MustElementX("//input[@id='Public IP Address']")
		enterPublicIPAddress1.MustInput(node2.Addr())
		enterAPIKey1 := page.MustElementX("//input[@id='API Key']")
		enterAPIKey1.MustInput(node2.APIKey())
		CreateButton1 := page.MustElement("div.container:nth-child(2)")
		CreateButton1.MustClick()
		newNodeID := page.MustElement("tr.table-item:nth-child(2) > th.align-left:nth-child(1)").MustText()
		require.Equal(t, node2.ID().String(), newNodeID)

	})
}
