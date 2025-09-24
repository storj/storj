// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

export class TeamPageObjects {
    static ADD_MEMBERS_BUTTON_XPATH = `//button[span[text()=' Add Members ']]`;
    static ADD_MEMBERS_TITLE_XPATH = `//div[text()='Add Member']`;
    static ADD_MEMBERS_EMAIL_INPUT_XPATH = `//input[@placeholder='Enter e-mail here']`;
    static ADD_MEMBERS_CONFIRM_BUTTON_XPATH = `//button[span[text()=' Send Invite ']]`;
    static TEAM_ROW_OWNER_CHIP_XPATH = `//td[span[div[text()='Owner']]]`;
    static TEAM_ROW_INVITED_CHIP_XPATH = `//td[span[div[text()='Invited']]]`;
    static TEAM_ROW_MEMBER_CHIP_XPATH = `//td[span[div[text()='Member']]]`;
    static JOIN_PROJECT_BUTTON_XPATH = `//button[span[text()=' Join Project ']]`;
    static CONFIRM_JOIN_PROJECT_BUTTON_XPATH = `//button[span[text()=' Join ']]`;
}
