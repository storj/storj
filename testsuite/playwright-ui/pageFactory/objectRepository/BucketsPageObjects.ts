// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

export class BucketsPageObjects {
    static ENCRYPTION_PASSPHRASE_XPATH = `//input[@id='Encryption Passphrase']`;
    static CONTINUE_BUTTON_PASSPHRASE_MODAL_XPATH = `//div[contains(@class, 'container') and contains(.//span, ' Continue ->')]`;
    static OBJECT_PREVIEW_BUTTON_XPATH = `//div[contains(@class, 'button-icon')]`;
    static OBJECT_MAP_IMAGE_XPATH = `//*[contains(@class, 'modal__map')]`;
    static COPY_LINK_BUTTON_XPATH = `//div[contains(@class, 'container') and contains(.//span, ' Copy Link')]`;
    static COPIED_TEXT = `Link Copied`;
    static NEW_FOLDER_BUTTON_TEXT = `New Folder`;
    static NEW_FOLDER_NAME_FIELD_XPATH = `//input[@id='Folder name']`;
    static CREATE_FOLDER_BUTTON_TEXT = `Create Folder`;
    static DELETE_BUTTON_XPATH = `//p[contains(text(),'Delete')]`;
    static YES_BUTTON_XPATH = `//*[contains(@class, 'delete-confirmation__options__item yes')]`;
    static VIEW_BUCKET_DETAILS_BUTTON_CSS = `.bucket-settings-nav__dropdown__item__label`;
    static BUCKET_SETTINGS_BUTTON_CSS = `.bucket-settings-nav`;
    static SHARE_BUCKET_BUTTON_XPATH = '//p[contains(text(),\'Share bucket\')]';

    // Create new bucket flow
    static NEW_BUCKET_BUTTON_XPATH = `//p[contains(text(),'New Bucket')]`;
    static BUCKET_NAME_INPUT_FIELD_XPATH = `//input[@id='Bucket Name']`;
    static CONTINUE_BUTTON_CREATE_BUCKET_FLOW_XPATH = `//div[contains(@class, 'container') and contains(.//span, ' Create bucket')]`;
    static ENTER_PASSPHRASE_RADIO_BUTTON_XPATH = `//h4[contains(text(),'Enter passphrase')]`;
    static PASSPHRASE_INPUT_NEW_BUCKET_XPATH = `//input[@id='Your Passphrase']`;
    static CHECKMARK_ENTER_PASSPHRASE_XPATH = `//label[contains(text(),'I understand, and I have saved the passphrase.')]`;
    static BUCKET_NAME_DELETE_BUCKET_MODAL_XPATH = `//input[@id='Type the name of the bucket to confirm']`;
    static CONFIRM_DELETE_BUTTON_XPATH = `//div[contains(@class, 'container') and contains(.//span, 'Delete Bucket')]`;
    static DELETE_BUCKET_XPATH = `//p[contains(text(),'Delete Bucket')]`;
}

export class BucketsPageObjectsV2 {
    static NEW_BUCKET_BUTTON_XPATH = `//button[span[text()=' New Bucket ']]`;
    static BUCKET_NAME_INPUT_FIELD_XPATH = `//input[@id='Bucket Name']`;
    static CONFIRM_BUTTON_CREATE_BUCKET_FLOW_XPATH = `//button[span[text()=' Create Bucket ']]`;
    static BUCKET_ROW_MORE_BUTTON_XPATH = `//button[@title='Bucket Actions']`;
    static VIEW_BUCKET_DETAILS_BUTTON_XPATH = `//div[div[div[text()=' Bucket Details ']]]`;
    static SHARE_BUCKET_BUTTON_XPATH = `//div[div[div[text()=' Share Bucket ']]]`;
    static DELETE_BUCKET_BUTTON_XPATH = `//div[div[div[text()=' Delete Bucket ']]]`;
    static CONFIRM_BUTTON_DELETE_BUCKET_FLOW_XPATH = `//button[span[text()=' Delete ']]`;
}
