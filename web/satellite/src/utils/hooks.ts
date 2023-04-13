// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

import { getCurrentInstance } from 'vue';
import VueRouter from 'vue-router';

import { Notificator } from '@/utils/plugins/notificator';

// TODO: remove after updating router and store deps.
export function useRouter() {
    return getCurrentInstance()?.proxy.$router || {} as VueRouter;
}

export function useNotify() {
    return getCurrentInstance()?.proxy.$notify || {} as Notificator;
}
