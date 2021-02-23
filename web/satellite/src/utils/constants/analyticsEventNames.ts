// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

export enum SegmentEvent {
    PROJECT_CREATED = 'Project Created',
    PROJECT_DELETED = 'Project Deleted',
    PROJECT_VIEWED = 'Project Viewed',
    USER_DELETED = 'User Deleted',
    USER_LOGGED_IN = 'User Logged In',
    EMAIL_VERIFIED = 'Email Verified',
    API_KEY_CREATED = 'API Key Created',
    API_KEY_DELETED = 'API Key Deleted',
    API_KEYS_VIEWED = 'API Key Viewed',
    PAYMENT_METHODS_VIEWED = 'Payment Methods Viewed',
    PAYMENT_METHOD_ADDED = 'Payment Method Added',
    REPORT_DOWNLOADED = 'Report Downloaded',
    REPORT_VIEWED = 'Report Viewed',
    BILLING_HISTORY_VIEWED = 'Billing History Viewed',
    TEAM_MEMBER_INVITED = 'Team Member Invited',
    TEAM_VIEWED = 'Team Viewed',
    CLI_DOCS_VIEWED = 'Uplink CLI Docs Viewed',
}
