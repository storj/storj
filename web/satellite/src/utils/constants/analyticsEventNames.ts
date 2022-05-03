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
    BANNER_ADD_CARD = 'Credit Card Added Through Banner',
    BANNER_ADD_TOKENS = 'Storj Token Added Through Banner',
}
