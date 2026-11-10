// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { ref } from 'vue';

export function useLoading() {
    const isLoading = ref<boolean>(false);

    /**
     * Runs callback unless a previous call is still loading, so that
     * repeated triggers (e.g. double clicks on submit) are dropped.
     */
    async function withLoading(callback): Promise<void> {
        if (isLoading.value) return;

        isLoading.value = true;
        try {
            await callback();
        } finally {
            isLoading.value = false;
        }
    }

    return {
        isLoading,
        withLoading,
    };
}

/**
 * Like useLoading, but for fetches whose parameters can change while loading:
 * calls aren't dropped while a previous one is still loading, and the results
 * of superseded calls can be ignored instead.
 */
export function useLatestLoading() {
    const isLoading = ref<boolean>(false);
    let latestCallID = 0;

    /**
     * Runs callback even if a previous call is still loading. callback receives
     * isLatest, which reports whether no newer call has started since, so that
     * results of superseded calls can be ignored. isLoading stays set until
     * the latest call finishes.
     */
    async function withLatestLoading(callback: (isLatest: () => boolean) => Promise<void>): Promise<void> {
        const callID = ++latestCallID;
        const isLatest = () => callID === latestCallID;

        isLoading.value = true;
        try {
            await callback(isLatest);
        } finally {
            if (isLatest()) isLoading.value = false;
        }
    }

    return {
        isLoading,
        withLatestLoading,
    };
}
