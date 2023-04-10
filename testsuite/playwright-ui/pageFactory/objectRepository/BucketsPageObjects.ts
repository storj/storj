export class BucketsPageObjects {
    protected static ENCRYPTION_PASSPHRASE_XPATH = `//input[@id='Encryption Passphrase']`;
    protected static CONTINUE_BUTTON_PASSPHRASE_MODAL_XPATH = `//span[contains(text(),'Continue ->')]`;
    protected static DOWNLOAD_BUTTON_XPATH = `//span[contains(text(),'Download')]`;
    protected static SHARE_BUTTON_XPATH = ` //span[contains(text(),'Share')]`;
    protected static DOWNLOAD_NOTIFICATION = `//p[contains(text(),'Do not share download link with other people. If you want to share this data bet')]`;
    protected static OBJECT_MAP_TEXT_XPATH = `//div[contains(text(),'Nodes storing this file')]`;
    protected static OBJECT_MAP_IMAGE_XPATH = `//*[contains(@class, 'object-map')]`;

    protected static COPY_LINK_BUTTON_XPATH = `//span[contains(text(),'Copy Link')]`;
    protected static COPIED_BUTTON_XPATH = `//span[contains(text(),'Copied!')]`;
    protected static CLOSE_MODAL_BUTTON_XPATH = `.mask__wrapper__container__close`;
    protected static NEW_FOLDER_BUTTON_XPATH = `//*[contains(text(),'New Folder')]`;
    protected static NEW_FOLDER_NAME_FIELD_XPATH = `//input[@id='Folder name']`;
    protected static CREATE_FOLDER_BUTTON_XPATH = `//span[contains(text(),'Create Folder')]`;
    protected static DELETE_BUTTON_XPATH = `//p[contains(text(),'Delete')]`;
    protected static YES_BUTTON_XPATH = `//*[contains(@class, 'delete-confirmation__options__item yes')]`;
    protected static VIEW_BUCKET_DETAILS_BUTTON_CSS = `.bucket-settings-nav__dropdown__item`;
    protected static BUCKET_SETTINGS_BUTTON_CSS = `.bucket-settings-nav`;
    protected static SHARE_BUCKET_BUTTON_XPATH = '//p[contains(text(),\'Share bucket\')]';
    protected static COPY_BUTTON_SHARE_BUCKET_MODAL_XPATH = `//span[contains(text(),'Copy')]`;

    // Create new bucket flow
    protected static NEW_BUCKET_BUTTON_XPATH = `//p[contains(text(),'New Bucket')]`;
    protected static BUCKET_NAME_INPUT_FIELD_XPATH = `//input[@id='Bucket Name']`;
    protected static CONTINUE_BUTTON_CREATE_BUCKET_FLOW_XPATH = `//span[contains(text(),'Create bucket')]`;
    protected static ENTER_PASSPHRASE_RADIO_BUTTON_XPATH = `//h4[contains(text(),'Enter passphrase')]`;
    protected static PASSPHRASE_INPUT_NEW_BUCKET_XPATH = `//input[@id='Your Passphrase']`;
    protected static CHECKMARK_ENTER_PASSPHRASE_XPATH = `//label[contains(text(),'I understand, and I have saved the passphrase.')]`;
    protected static BUCKET_NAME_DELETE_BUCKET_MODAL_XPATH = `//input[@id='Bucket Name']`;
    protected static CONFIRM_DELETE_BUTTON_XPATH = `//span[contains(text(),'Confirm Delete Bucket')]`
}
