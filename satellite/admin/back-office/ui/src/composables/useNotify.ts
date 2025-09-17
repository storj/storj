// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { inject } from 'vue';

import { Notificator } from '@/plugins/notificator';

export function useNotify() {
    return inject('notify') as Notificator;
}
