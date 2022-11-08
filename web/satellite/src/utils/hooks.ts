// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

import { getCurrentInstance } from 'vue';

// TODO: remove after updating router and store deps.
export function useRoute() {
    return getCurrentInstance()?.proxy.$route;
}

export function useRouter() {
    return getCurrentInstance()?.proxy.$router;
}

export function useStore() {
    return getCurrentInstance()?.proxy.$store;
}

export function useNotify() {
    return getCurrentInstance()?.proxy.$notify;
}
