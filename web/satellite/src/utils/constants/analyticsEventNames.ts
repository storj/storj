// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Make sure these event names match up with the client-side event names in satellite/analytics/service.go
export enum AnalyticsEvent {
    GATEWAY_CREDENTIALS_CREATED = 'Credentials Created',
    PASSPHRASE_CREATED = 'Passphrase Created',
    EXTERNAL_LINK_CLICKED = 'External Link Clicked',
    PATH_SELECTED = 'Path Selected',
    LINK_SHARED = 'Link Shared',
    OBJECT_UPLOADED = 'Object Uploaded',
    API_KEY_GENERATED = 'API Key Generated',
    UPGRADE_BANNER_CLICKED = 'Upgrade Banner Clicked',
    MODAL_ADD_CARD = 'Credit Card Added In Modal',
    MODAL_ADD_TOKENS = 'Storj Token Added In Modal',
    SEARCH_BUCKETS = 'Search Buckets',
    NAVIGATE_PROJECTS = 'Navigate Projects',
    MANAGE_PROJECTS_CLICKED = 'Manage Projects Clicked',
    CREATE_NEW_CLICKED = 'Create New Clicked',
    VIEW_DOCS_CLICKED = 'View Docs Clicked',
    VIEW_FORUM_CLICKED = 'View Forum Clicked',
    VIEW_SUPPORT_CLICKED = 'View Support Clicked',
    CREATE_AN_ACCESS_GRANT_CLICKED = 'Create an Access Grant Clicked',
    UPLOAD_USING_CLI_CLICKED = 'Upload Using CLI Clicked',
    UPLOAD_IN_WEB_CLICKED = 'Upload In Web Clicked',
    NEW_PROJECT_CLICKED = 'New Project Clicked',
    LOGOUT_CLICKED = 'Logout Clicked',
    PROFILE_UPDATED = 'Profile Updated',
    PASSWORD_CHANGED = 'Password Changed',
    MFA_ENABLED = 'MFA Enabled',
    BUCKET_CREATED = 'Bucket Created',
    BUCKET_DELETED = 'Bucket Deleted',
}
