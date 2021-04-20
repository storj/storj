// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Make sure these event names match up with the client-side event names in satellite/analytics/service.go
export enum AnalyticsEvent {
    GATEWAY_CREDENTIALS_CREATED = 'Credentials Created',
    PASSPHRASE_CREATED = 'Passphrase Created',
    EXTERNAL_LINK_CLICKED = 'External Link Clicked',
    PATH_SELECTED = 'Path Selected',
}
