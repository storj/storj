// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { Ref, ref } from 'vue';

export function useLoading() {
    const isLoading = ref<boolean>(false);

    async function withLoading(callback): Promise<void> {
        if (isLoading.value) return;

        isLoading.value = true;

        await callback();

        isLoading.value = false;
    }

    async function withCustomLoading(loading: Ref<boolean>, callback: () => Promise<unknown>): Promise<void> {
        if (loading.value) return;

        loading.value = true;

        await callback();

        loading.value = false;
    }

    return {
        isLoading,
        withLoading,
        withCustomLoading,
    };
}
