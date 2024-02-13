// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

import { App } from 'vue';

import { noWindowTouchInDialog } from '@/mixins/noWindowTouchInDialog';

export function registerMixins(app: App<Element>): void {
    app.mixin(noWindowTouchInDialog);
}
