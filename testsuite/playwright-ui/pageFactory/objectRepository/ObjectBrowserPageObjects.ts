// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

export class ObjectBrowserPageObjects {
    static MAIN_VIEW_CLASS = `.bucket-view`;
    static BREADCRUMB_CLASS = `.v-breadcrumbs-item`;
    static TABLE_ROW_SELECTOR = `tbody tr:not(.v-data-table-rows-no-data)`;
    static DOWNLOAD_BUTTON_XPATH = `//button[@id='Download']`;
    static DISTRIBUTION_BUTTON_XPATH = `//button[@id='Distribution']`;
    static SHARE_BUTTON_XPATH = `//button[@id='Share']`;
    static COPY_LINK_BUTTON_XPATH = `//button[span[text()='Copy Link']]`;
    static COPY_ICON_BUTTON = `[aria-roledescription="copy-btn"]`;
    static COPIED_TEXT = `Copied`;
    static SHARE_MODAL_NEXT_BUTTON_XPATH = `//button[span[text()=' Next -> ']]`;
    static SHARE_MODAL_PREVIEW_LINK_TITLE_XPATH = `//div/div/div/p[contains(text(),' Interactive Preview Link ')]`;
    static OBJECT_MAP_IMAGE_XPATH = `//img[@id='Map']`;
    static DELETE_ROW_ACTION_BUTTON_XPATH = `//div[div[div[text()=' Delete ']]]`;
    static SNACKBAR_DELETE_BUTTON_SELECTOR = `.v-snackbar button:has-text("Delete")`;
    static CONFIRM_DELETE_BUTTON_SELECTOR = `.v-dialog button:has-text("Delete")`;
    static FILE_INPUT_XPATH = `//input[@id='File Input']`;
    static FOLDER_INPUT_XPATH = `//input[@id='Folder Input']`;
    static LOADING_ITEMS_LABEL_XPATH = `//td[text()='Loading items...']`;
    static CREATE_FOLDER_BUTTON_XPATH = `//button[span[text()=' New Folder ']]`;
    static FOLDER_NAME_INPUT_XPATH = `//input[@id='Folder Name']`;
    static CONFIRM_CREATE_FOLDER_BUTTON_XPATH = `//button[span[text()=' Create Folder ']]`;
    static CLOSE_SHARE_MODAL_BUTTON_XPATH = `//button[@id='close-share']`;
    static CLOSE_GEO_DIST_MODAL_BUTTON_XPATH = `//button[@id='close-geo-distribution']`;
    static CLOSE_PREVIEW_MODAL_BUTTON_XPATH = `//button[@id='close-preview']`;
}
