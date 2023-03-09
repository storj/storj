// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { ref } from 'vue';

export function useLoading() {
    const isLoading = ref<boolean>(false);

    async function withLoading(callback): Promise<void> {
        if (isLoading.value) return;

        isLoading.value = true;

        await callback();

        isLoading.value = false;
    }

    return {
        isLoading,
        withLoading,
    };
}
