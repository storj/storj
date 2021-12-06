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

func TestDropdownMoreOptionsMultinode(t *testing.T) {
	uitest.Multinode(t, 1, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser) {
		startPage := planet.Multinodes[0].ConsoleURL() + "/add-first-node"
		page := openPage(browser, startPage)

		node := planet.StorageNodes[0]

		page.MustElement("input#Node\\ ID.headered-input").MustInput(node.ID().String())
		page.MustElement("input#Public\\ IP\\ Address.headered-input").MustInput(node.Addr())
		page.MustElement("input#API\\ Key.headered-input").MustInput(node.APIKey())

		addNodeButton := page.MustElementR(".add-first-node__left-area__button", "Add Node").MustClick()
		require.Equal(t, "Add Node", addNodeButton.MustText())
		page.MustWaitNavigation()

		nodesPageTitle := page.MustElement("h1.my-nodes__title").MustText()
		require.Equal(t, "My Nodes", nodesPageTitle)

		// Test- Change Node Display Name
		moreOptionsButton0 := page.MustElement("div.options-button > svg:nth-child(1)")
		moreOptionsButton0.MustClick()
		page.MustElement("div.update-name__button").MustClick()
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

		// Test- Cancel Change Node Display Name with X Button
		moreOptionsButton1 := page.MustElement("div.options-button > svg:nth-child(1)")
		moreOptionsButton1.MustClick()
		page.MustElement("div.update-name__button").MustClick()
		page.MustElementX("//input[@id='Displayed name']").MustInput("NEWNAME")
		xButton0 := page.MustElement("div.modal__cross ")
		xButton0.MustClick()
		require.Equal(t, "newnodename", newNodeDisplay)

		// Test- Cancel Change Node Display Name with Cancel Button
		moreOptionsButton2 := page.MustElement("div.options-button > svg:nth-child(1)")
		moreOptionsButton2.MustClick()
		page.MustElement("div.update-name__button").MustClick()
		page.MustElementX("//input[@id='Displayed name']").MustInput("NEWNAME")
		cancelButton0 := page.MustElement("div.container.white")
		cancelButton0.MustClick()
		require.Equal(t, "newnodename", newNodeDisplay)

		// Test Cancel Delete Node Option with X Button
		moreOptionsButton3 := page.MustElement("div.options-button > svg:nth-child(1)")
		moreOptionsButton3.MustClick()
		deleteNodeButton0 := page.MustElement("div.delete-node__button")
		deleteNodeButton0.MustClick()
		deleteNodeTitle0 := page.MustElement("div.modal__header").MustText()
		require.Equal(t, "Delete this node?", deleteNodeTitle0)
		xButton1 := page.MustElement("div.modal__cross")
		xButton1.MustClick()
		require.Equal(t, "newnodename", newNodeDisplay)

		// Test Cancel Delete Node Option with Cancel Button
		moreOptionsButton4 := page.MustElement("div.options-button > svg:nth-child(1)")
		moreOptionsButton4.MustClick()
		deleteNodeButton1 := page.MustElement("div.delete-node__button")
		deleteNodeButton1.MustClick()
		deleteNodeTitle1 := page.MustElement("div.modal__header").MustText()
		require.Equal(t, "Delete this node?", deleteNodeTitle1)
		cancelButton1 := page.MustElement("div.container.white")
		cancelButton1.MustClick()
		require.Equal(t, "newnodename", newNodeDisplay)

		// Test Delete Node Option
		moreOptionsButton6 := page.MustElement("div.options-button > svg:nth-child(1)")
		moreOptionsButton6.MustClick()
		deleteNodeButton2 := page.MustElement("div.delete-node__button")
		deleteNodeButton2.MustClick()
		deleteNodeTitle2 := page.MustElement("div.modal__header").MustText()
		require.Equal(t, "Delete this node?", deleteNodeTitle2)
		deleteButton := page.MustElement("div.container:nth-child(2)")
		deleteButton.MustClick()
		require.Equal(t, "My Nodes", nodesPageTitle)

		// Test- Copy Node ID, first node has to be added back
		newNodeButton := page.MustElement("div.container > span.label")
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
		// Then it clicks on the more options for nodes and node ID is copied
		moreOptionsButton5 := page.MustElement(".options-button")
		moreOptionsButton5.MustClick()
		dropDownCopyNodeID := page.MustElement("div.options:nth-child(2) > div.options__item")
		dropDownCopyNodeID.MustClick()
		// Clicks on more options again and then update node name
		moreOptionsButton5.MustClick()
		dropDownUpdateName := page.MustElement("div.update-name__button")
		dropDownUpdateName.MustClick()
		// Paste node ID to input and checks if it matches current name (default nodeID)
		input := page.MustElementX("//input[@id='Displayed name']")
		input.MustPress('\u0408')
		copiedInput := page.MustElementX("//input[@id='Displayed name']").MustText()
		nodeID := page.MustElement("div.update-name__body__node-id-container > span:nth-child(1)").MustText()
		require.Equal(t, nodeID, copiedInput)

	})
}
