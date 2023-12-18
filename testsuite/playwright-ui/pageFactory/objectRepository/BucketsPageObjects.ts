// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

export class BucketsPageObjects {
    static ENCRYPTION_PASSPHRASE_XPATH = `//input[@id='Encryption Passphrase']`;
    static CONTINUE_BUTTON_PASSPHRASE_MODAL_XPATH = `//div[contains(@class, 'container') and contains(.//span, ' Continue ->')]`;
    static DOWNLOAD_BUTTON_XPATH = `//div[contains(@class, 'container') and contains(.//span, ' Download')]`;
    static SHARE_BUTTON_XPATH = `div:nth-child(4) > .button-icon`;
    static DOWNLOAD_NOTIFICATION = `//p[contains(text(),'Keep this download link private.If you want to share, use the Share option.')]`;
    static OBJECT_MAP_TEXT_XPATH = `//div[contains(text(),'Nodes storing this file')]`;
    static OBJECT_MAP_IMAGE_XPATH = `//*[contains(@class, 'object-map')]`;
    static COPY_LINK_BUTTON_XPATH = `//div[contains(@class, 'container') and contains(.//span, ' Copy Link')]`;
    static COPIED_TEXT = `Copied!`;
    static CLOSE_MODAL_BUTTON_XPATH = `.mask__wrapper__container__close`;
    static CLOSE_FILE_PREVIEW_BUTTON_XPATH = `//body/div[@id='app']/div[2]/div[1]/div[2]/div[5]/div[1]`;
    static NEW_FOLDER_BUTTON_TEXT = `New Folder`;
    static NEW_FOLDER_NAME_FIELD_XPATH = `//input[@id='Folder name']`;
    static CREATE_FOLDER_BUTTON_TEXT = `Create Folder`;
    static DELETE_BUTTON_XPATH = `//p[contains(text(),'Delete')]`;
    static YES_BUTTON_XPATH = `//*[contains(@class, 'delete-confirmation__options__item yes')]`;
    static VIEW_BUCKET_DETAILS_BUTTON_CSS = `.bucket-settings-nav__dropdown__itprivateem`;
    static BUCKET_SETTINGS_BUTTON_CSS = `.bucket-settings-nav`;
    static SHARE_BUCKET_BUTTON_XPATH = '//p[contains(text(),\'Share bucket\')]';
    static COPY_BUTTON_SHARE_BUCKET_MODAL_XPATH = `//span[contains(text(),'Copy')]`;

    // Create new bucket flow
    static NEW_BUCKET_BUTTON_XPATH = `//p[contains(text(),'New Bucket')]`;
    static BUCKET_NAME_INPUT_FIELD_XPATH = `//input[@id='Bucket Name']`;
    static CONTINUE_BUTTON_CREATE_BUCKET_FLOW_XPATH = `//div[contains(@class, 'container') and contains(.//span, ' Create bucket')]`;
    static ENTER_PASSPHRASE_RADIO_BUTTON_XPATH = `//h4[contains(text(),'Enter passphrase')]`;
    static PASSPHRASE_INPUT_NEW_BUCKET_XPATH = `//input[@id='Your Passphrase']`;
    static CHECKMARK_ENTER_PASSPHRASE_XPATH = `//label[contains(text(),'I understand, and I have saved the passphrase.')]`;
    static BUCKET_NAME_DELETE_BUCKET_MODAL_XPATH = `//input[@id='Bucket Name']`;
    static CONFIRM_DELETE_BUTTON_XPATH = `//div[contains(@class, 'container') and contains(.//span, ' Confirm Delete Bucket')]`;
    static DELETE_BUCKET_XPATH = `//p[contains(text(),'Delete Bucket')]`;
}
