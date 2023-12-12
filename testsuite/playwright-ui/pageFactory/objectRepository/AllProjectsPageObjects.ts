// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

export class AllProjectsPageObjects {
    protected static ALL_PROJECTS_HEADER_TITLE_XPATH = `//span[contains(text(),'My Projects')]`;
    protected static OPEN_PROJECT_BUTTON_TEXT = `Open Project`;
    protected static PROJECT_ITEM_XPATH = `//*[contains(@class, \'project-item\')]`;
}
