// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { getCurrentInstance } from 'vue';

import { store } from '@/app/store';

// TODO: remove after migration.
export function useRoute() {
    return getCurrentInstance()?.proxy.$route;
}

export function useRouter() {
    return getCurrentInstance()?.proxy.$router;
}

export function useStore() {
    return getCurrentInstance()?.proxy.$store ?? {} as typeof store;
}

export function useVuetify() {
    return getCurrentInstance()?.proxy.$vuetify;
}
