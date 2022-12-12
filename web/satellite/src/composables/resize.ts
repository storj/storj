// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

import { computed, onMounted, onUnmounted, ref } from 'vue';

export function useResize() {
    const screenWidth = ref<number>(window.innerWidth);
    const screenHeight = ref<number>(window.innerHeight);

    const isMobile = computed((): boolean => {
        return screenWidth.value <= 550;
    });

    const isTablet = computed((): boolean => {
        return !isMobile.value && screenWidth.value <= 800;
    });

    function onResize(): void {
        screenWidth.value = window.innerWidth;
        screenHeight.value = window.innerHeight;
    }

    onMounted((): void => {
        window.addEventListener('resize', onResize);
    });

    onUnmounted((): void => {
        window.removeEventListener('resize', onResize);
    });

    return {
        screenWidth,
        screenHeight,
        isMobile,
        isTablet,
    };
}
