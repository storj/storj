// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

export class ObjectBrowserPageObjects {
    static DOWNLOAD_BUTTON_XPATH = `//button[@id='Download']`;
    static DISTRIBUTION_BUTTON_XPATH = `//button[@id='Distribution']`;
    static SHARE_BUTTON_XPATH = `//button[@id='Share']`;
    static COPY_LINK_BUTTON_XPATH = `//button[span[text()='Copy Link']]`;
    static COPIED_TEXT = `Copied`;
    static SHARE_MODAL_LOADER_CLASS = `.share-dialog__content--loading`;
    static OBJECT_MAP_IMAGE_XPATH = `//img[@id='Map']`;
    static OBJECT_ROW_MORE_BUTTON_XPATH = `//button[@title='More Actions']`;
    static DELETE_ROW_ACTION_BUTTON_XPATH = `//div[div[div[text()=' Delete ']]]`;
    static CONFIRM_DELETE_BUTTON_XPATH = `//button[span[text()=' Delete ']]`;
    static FILE_INPUT_XPATH = `//input[@id='File Input']`;
    static FOLDER_INPUT_XPATH = `//input[@id='Folder Input']`;
    static LOADING_ITEMS_LABEL_XPATH = `//td[text()='Loading items...']`;
    static CREATE_FOLDER_BUTTON_XPATH = `//button[span[text()=' New Folder ']]`;
    static FOLDER_NAME_INPUT_XPATH = `//input[@id='Folder Name']`;
    static CONFIRM_CREATE_FOLDER_BUTTON_XPATH = `//button[span[text()=' Create Folder ']]`;
    static CLOSE_SHARE_MODAL_BUTTON_XPATH = `//button[@id='close-share']`
    static CLOSE_GEO_DIST_MODAL_BUTTON_XPATH = `//button[@id='close-geo-distribution']`
    static CLOSE_PREVIEW_MODAL_BUTTON_XPATH = `//button[@id='close-preview']`
}
