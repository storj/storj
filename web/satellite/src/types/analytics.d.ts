// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

declare type analytics = {
    page(name: string, options?: object): void,
    track(event: string, options?: object): void,
    identity(userId: string, options?: object): void,
}