// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

// Make sure these event names match up with the client-side event names in satellite/analytics/service.go
export enum AnalyticsEvent {
    GATEWAY_CREDENTIALS_CREATED = 'Credentials Created',
    LINK_SHARED = 'Link Shared',
    OBJECT_UPLOADED = 'Object Uploaded',
    MODAL_ADD_CARD = 'Credit Card Added In Modal',
    SEARCH_BUCKETS = 'Search Buckets',
    NAVIGATE_PROJECTS = 'Navigate Projects',
    VIEW_DOCS_CLICKED = 'View Docs Clicked',
    VIEW_FORUM_CLICKED = 'View Forum Clicked',
    VIEW_SUPPORT_CLICKED = 'View Support Clicked',
    NEW_PROJECT_CLICKED = 'New Project Clicked',
    LOGOUT_CLICKED = 'Logout Clicked',
    PROFILE_UPDATED = 'Profile Updated',
    PASSWORD_CHANGED = 'Password Changed',
    MFA_ENABLED = 'MFA Enabled',
    BUCKET_CREATED = 'Bucket Created',
    BUCKET_DELETED = 'Bucket Deleted',
    ACCESS_GRANT_CREATED = 'Access Grant Created',
    API_ACCESS_CREATED = 'API Access Created',
    UPLOAD_FILE_CLICKED = 'Upload File Clicked',
    UPLOAD_FOLDER_CLICKED = 'Upload Folder Clicked',
    DOWNLOAD_TXT_CLICKED = 'Download txt clicked',
    COPY_TO_CLIPBOARD_CLICKED = 'Copy to Clipboard Clicked',
    COUPON_CODE_APPLIED = 'Coupon Code Applied',
    CREDIT_CARD_ADDED_FROM_BILLING = 'Credit Card Added From Billing',
    ADD_FUNDS_CLICKED = 'Add Funds Clicked',
    PROJECT_MEMBERS_INVITE_SENT = 'Project Members Invite Sent',
    UI_ERROR = 'UI error occurred',
    PROJECT_NAME_UPDATED = 'Project Name Updated',
    PROJECT_DESCRIPTION_UPDATED = 'Project Description Updated',
    PROJECT_STORAGE_LIMIT_UPDATED = 'Project Storage Limit Updated',
    PROJECT_BANDWIDTH_LIMIT_UPDATED = 'Project Bandwidth Limit Updated',
    PROJECT_INVITATION_ACCEPTED = 'Project Invitation Accepted',
    PROJECT_INVITATION_DECLINED = 'Project Invitation Declined',
    PASSPHRASE_CREATED = 'Passphrase Created',
    RESEND_INVITE_CLICKED = 'Resend Invite Clicked',
    COPY_INVITE_LINK_CLICKED = 'Copy Invite Link Clicked',
    USER_SIGN_UP = 'User Sign Up',
    PERSONAL_INFO_SUBMITTED = 'Personal Info Submitted',
    BUSINESS_INFO_SUBMITTED = 'Business Info Submitted',
    USE_CASE_SELECTED = 'Use Case Selected',
    ONBOARDING_COMPLETED = 'Onboarding Completed',
    ONBOARDING_ABANDONED = 'Onboarding Abandoned',
    PERSONAL_SELECTED = 'Personal Selected',
    BUSINESS_SELECTED = 'Business Selected',
    UPGRADE_CLICKED = 'Upgrade Clicked',
    ARRIVED_FROM_SOURCE = 'Arrived From Source',
    APPLICATIONS_SETUP_CLICKED = 'Applications Setup Clicked',
    APPLICATIONS_SETUP_COMPLETED = 'Applications Setup Completed',
    APPLICATIONS_DOCS_CLICKED = 'Applications Docs Clicked',
    CLOUD_GPU_NAVIGATION_ITEM_CLICKED = 'Cloud GPU Navigation Item Clicked',
    CLOUD_GPU_SIGN_UP_CLICKED = 'Cloud GPU Sign Up Clicked',
    JOIN_CUNO_FS_BETA_FORM_SUBMITTED = 'Join CunoFS Beta Form Submitted',
    OBJECT_MOUNT_CONSULTATION_SUBMITTED = 'Object Mount Consultation Submitted',
}

export enum AnalyticsErrorEventSource {
    ACCESS_GRANTS_WEB_WORKER = 'Access grant web worker',
    ACCESS_GRANTS_PAGE = 'Access grants page',
    API_KEYS_PAGE = 'REST API keys page',
    CREATE_API_KEY_DIALOG = 'Create REST API keys dialog',
    ACCOUNT_PAGE = 'Account page',
    ACCOUNT_SETTINGS_AREA = 'Account settings area',
    ACCOUNT_SETUP_DIALOG = 'Account setup dialog',
    ACCOUNT_DELETE_DIALOG = 'Account delete dialog',
    PROJECT_DELETE_DIALOG = 'Project delete dialog',
    BILLING_HISTORY_TAB = 'Billing history tab',
    BILLING_PAYMENT_METHODS_TAB = 'Billing payment methods tab',
    BILLING_APPLY_COUPON_CODE_INPUT = 'Billing apply coupon code input',
    BILLING_STRIPE_CARD_INPUT = 'Billing stripe card input',
    BILLING_AREA = 'Billing area',
    BILLING_STORJ_TOKEN_CONTAINER = 'Billing STORJ token container',
    BUCKET_DETAILS_MODAL = 'Bucket details modal',
    SETUP_ACCESS_MODAL = 'Setup access modal',
    CONFIRM_DELETE_AG_MODAL = 'Confirm delete access grant modal',
    FILE_BROWSER_LIST_CALL = 'File browser - list API call',
    FILE_BROWSER_ENTRY = 'File browser entry',
    FILE_BROWSER = 'File browser',
    CUNO_FS_BETA_FORM = 'CunoFS beta form',
    OBJECT_MOUNT_CONSULTATION_FORM = 'Object Mount consultation form',
    UPGRADE_ACCOUNT_MODAL = 'Upgrade account modal',
    ADD_PROJECT_MEMBER_MODAL = 'Add project member modal',
    CHANGE_PASSWORD_MODAL = 'Change password modal',
    CHANGE_EMAIL_DIALOG = 'Change email dialog',
    CREATE_PROJECT_MODAL = 'Create project modal',
    CREATE_PROJECT_PASSPHRASE_MODAL = 'Create project passphrase modal',
    CREATE_BUCKET_MODAL = 'Create bucket modal',
    SET_BUCKET_OBJECT_LOCK_CONFIG_MODAL = 'Set bucket object lock config modal',
    DELETE_BUCKET_MODAL = 'Delete bucket modal',
    ENABLE_MFA_MODAL = 'Enable MFA modal',
    MFA_CODES_MODAL = 'MFA codes modal',
    DISABLE_MFA_MODAL = 'Disable MFA modal',
    EDIT_PROFILE_MODAL = 'Edit profile modal',
    CREATE_FOLDER_MODAL = 'Create folder modal',
    OPEN_BUCKET_MODAL = 'Open bucket modal',
    SHARE_MODAL = 'Share modal',
    OBJECTS_UPLOAD_MODAL = 'Objects upload modal',
    BUCKET_TABLE = 'Bucket table',
    UPLOAD_FILE_VIEW = 'Upload file view',
    GALLERY_VIEW = 'Gallery view',
    OBJECT_UPLOAD_ERROR = 'Object upload error',
    PROJECT_DASHBOARD_PAGE = 'Project dashboard page',
    PROJECT_SETTINGS_AREA = 'Project settings area',
    EDIT_PROJECT_DETAILS = 'Edit project details',
    EDIT_PROJECT_LIMIT = 'Edit project limit',
    PROJECT_MEMBERS_HEADER = 'Project members page header',
    PROJECT_MEMBERS_PAGE = 'Project members page',
    OVERALL_APP_WRAPPER_ERROR = 'Overall app wrapper error',
    OVERALL_SESSION_EXPIRED_ERROR = 'Overall session expired error',
    ALL_PROJECT_DASHBOARD = 'All projects dashboard error',
    EDIT_TIMEOUT_MODAL = 'Edit session timeout error',
    JOIN_PROJECT_MODAL = 'Join project modal',
    NEW_DOMAIN_MODAL = 'New domain modal',
    PROJECT_INVITATION = 'Project invitation',
    DETAILED_USAGE_REPORT_MODAL = 'Detailed usage report modal',
    REMOVE_CC_MODAL = 'Remove credit card modal',
    EDIT_DEFAULT_CC_MODAL = 'Edit default credit card modal',
    ONBOARDING_STEPPER = 'Onboarding stepper',
    VERSIONING_TOGGLE_DIALOG = 'Versioning toggle dialog',
    UPLOAD_OVERWRITE_WARNING_DIALOG = 'Upload Overwrite Warning Dialog',
    LOCK_OBJECT_DIALOG = 'Lock Object Dialog',
    LEGAL_HOLD_DIALOG = 'Legal Hold Dialog',
    ADD_FUNDS_DIALOG = 'Add Funds Dialog',
    DOWNLOAD_PREFIX_DIALOG = 'Download Prefix Dialog',
    APPLICATION_BAR = 'Application bar',
}

export enum PageVisitSource {
    DOCS = 'docs',
    FORUM = 'forum',
    SUPPORT = 'support',
    VALDI = 'valdi',
}

export const SATELLITE_MANAGED_ENCRYPTION_DOCS_PAGE = 'https://docs.storj.io/learn/concepts/encryption-key/storj-vs-user-managed-encryption';