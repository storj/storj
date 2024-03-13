// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

export class BucketsPageObjects {
    static NEW_BUCKET_BUTTON_XPATH = `//button[span[text()=' New Bucket ']]`;
    static BUCKET_NAME_INPUT_FIELD_XPATH = `//input[@id='Bucket Name']`;
    static CONFIRM_BUTTON_CREATE_BUCKET_FLOW_XPATH = `//button[span[text()='Create Bucket']]`;
    static BUCKET_ROW_MORE_BUTTON_XPATH = `//button[@title='Bucket Actions']`;
    static VIEW_BUCKET_DETAILS_BUTTON_XPATH = `//div[div[div[text()=' Bucket Details ']]]`;
    static SHARE_BUCKET_BUTTON_XPATH = `//div[div[div[text()=' Share Bucket ']]]`;
    static DELETE_BUCKET_BUTTON_XPATH = `//div[div[div[text()=' Delete Bucket ']]]`;
    static CONFIRM_BUTTON_DELETE_BUCKET_FLOW_XPATH = `//button[span[text()=' Delete ']]`;
}
