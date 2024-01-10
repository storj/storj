// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

export class AllProjectsPageObjects {
    static ALL_PROJECTS_HEADER_TITLE_XPATH = `//span[contains(text(),'My Projects')]`;
    static OPEN_PROJECT_BUTTON_TEXT = `Open Project`;
    static PROJECT_ITEM_XPATH = `//*[contains(@class, 'project-item')]`;
    static CREATE_PROJECT_BUTTON_XPATH = `//div[contains(@class, 'container') and contains(.//span, ' Create a Project')]`;
    static CONFIRM_CREATE_PROJECT_BUTTON_XPATH = `//div[contains(@class, 'container') and contains(.//span, ' Create Project -->')]`;
    static NEW_PROJECT_NAME_FIELD_XPATH = `//input[@id='Project Name']`;
    static SKIP_ONBOARDING_LABEL = ` Skip and go directly to dashboard `;
}

export class AllProjectsPageObjectsV2 {
    static CREATE_PROJECT_BUTTON_XPATH = `//button[span[text()=' Create Project ']]`;
    static CONFIRM_CREATE_PROJECT_BUTTON_XPATH = `//button[span[text()='Create Project']]`;
    static NEW_PROJECT_NAME_FIELD_XPATH = `//input[@id='Project Name']`;
}
