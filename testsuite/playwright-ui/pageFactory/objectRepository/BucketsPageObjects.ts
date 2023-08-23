// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

export class BucketsPageObjects {
    protected static ENCRYPTION_PASSPHRASE_XPATH = `//input[@id='Encryption Passphrase']`;
    protected static CONTINUE_BUTTON_PASSPHRASE_MODAL_XPATH = `//div[contains(@class, 'container') and contains(.//span, ' Continue ->')]`;
    protected static DOWNLOAD_BUTTON_XPATH = `//div[contains(@class, 'container') and contains(.//span, ' Download')]`;
    protected static SHARE_BUTTON_XPATH = `div:nth-child(4) > .button-icon`;
    protected static DOWNLOAD_NOTIFICATION = `//p[contains(text(),'Keep this download link private.If you want to share, use the Share option.')]`;
    protected static OBJECT_MAP_TEXT_XPATH = `//div[contains(text(),'Nodes storing this file')]`;
    protected static OBJECT_MAP_IMAGE_XPATH = `//*[contains(@class, 'object-map')]`;
    protected static COPY_LINK_BUTTON_XPATH = `//div[contains(@class, 'container') and contains(.//span, ' Copy Link')]`;
    protected static COPIED_TEXT = `Copied!`;
    protected static CLOSE_MODAL_BUTTON_XPATH = `.mask__wrapper__container__close`;
    protected static CLOSE_FILE_PREVIEW_BUTTON_XPATH = `//body/div[@id='app']/div[2]/div[1]/div[2]/div[5]/div[1]`
    protected static NEW_FOLDER_BUTTON_TEXT = `New Folder`;
    protected static NEW_FOLDER_NAME_FIELD_XPATH = `//input[@id='Folder name']`;
    protected static CREATE_FOLDER_BUTTON_TEXT = `Create Folder`;
    protected static DELETE_BUTTON_XPATH = `//p[contains(text(),'Delete')]`;
    protected static YES_BUTTON_XPATH = `//*[contains(@class, 'delete-confirmation__options__item yes')]`;
    protected static VIEW_BUCKET_DETAILS_BUTTON_CSS = `.bucket-settings-nav__dropdown__item`;
    protected static BUCKET_SETTINGS_BUTTON_CSS = `.bucket-settings-nav`;
    protected static SHARE_BUCKET_BUTTON_XPATH = '//p[contains(text(),\'Share bucket\')]';
    protected static COPY_BUTTON_SHARE_BUCKET_MODAL_XPATH = `//span[contains(text(),'Copy')]`;

    // Create new bucket flow
    protected static NEW_BUCKET_BUTTON_XPATH = `//p[contains(text(),'New Bucket')]`;
    protected static BUCKET_NAME_INPUT_FIELD_XPATH = `//input[@id='Bucket Name']`;
    protected static CONTINUE_BUTTON_CREATE_BUCKET_FLOW_XPATH = `//div[contains(@class, 'container') and contains(.//span, ' Create bucket')]`;
    protected static ENTER_PASSPHRASE_RADIO_BUTTON_XPATH = `//h4[contains(text(),'Enter passphrase')]`;
    protected static PASSPHRASE_INPUT_NEW_BUCKET_XPATH = `//input[@id='Your Passphrase']`;
    protected static CHECKMARK_ENTER_PASSPHRASE_XPATH = `//label[contains(text(),'I understand, and I have saved the passphrase.')]`;
    protected static BUCKET_NAME_DELETE_BUCKET_MODAL_XPATH = `//input[@id='Bucket Name']`;
    protected static CONFIRM_DELETE_BUTTON_XPATH = `//div[contains(@class, 'container') and contains(.//span, ' Confirm Delete Bucket')]`;
    protected static DELETE_BUCKET_XPATH = `//p[contains(text(),\'Delete Bucket\')]`
}
