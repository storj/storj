// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { getCurrentInstance } from 'vue';

// TODO: remove after migration.
export function useRoute() {
    return getCurrentInstance()?.proxy.$route;
}

export function useRouter() {
    return getCurrentInstance()?.proxy.$router;
}
