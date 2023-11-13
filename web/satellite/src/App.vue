// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div id="app">
        <BrandedLoader v-if="isLoading" />
        <ErrorPage v-else-if="isErrorPageShown" />
        <router-view v-else />
        <!-- Area for displaying notification -->
        <NotificationArea />
    </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';

import { useNotify } from '@/utils/hooks';
import { useAppStore } from '@/store/modules/appStore';
import { useConfigStore } from '@/store/modules/configStore';

import ErrorPage from '@/views/ErrorPage.vue';
import BrandedLoader from '@/components/common/BrandedLoader.vue';
import NotificationArea from '@/components/notifications/NotificationArea.vue';

const configStore = useConfigStore();
const appStore = useAppStore();
const notify = useNotify();

const isLoading = ref<boolean>(true);

/**
 * Indicates whether an error page should be shown in place of the router view.
 */
const isErrorPageShown = computed<boolean>((): boolean => {
    return appStore.state.error.visible;
});

/**
 * Fixes the issue where view port height is taller than the visible viewport on
 * mobile Safari/Webkit. See: https://bugs.webkit.org/show_bug.cgi?id=141832
 * Specifically for us, this issue is seen in Safari and Google Chrome, both on iOS
 */
function fixViewportHeight(): void {
    const agent = window.navigator.userAgent.toLowerCase();
    const isMobile = screen.width <= 500;
    const isIOS = agent.includes('applewebkit') && agent.includes('iphone');
    // We don't want to apply this fix on FxIOS because it introduces strange behavior
    // while not fixing the issue because it doesn't exist here.
    const isFirefoxIOS = window.navigator.userAgent.toLowerCase().includes('fxios');

    if (isMobile && isIOS && !isFirefoxIOS) {
        // Set the custom --vh variable to the root of the document.
        document.documentElement.style.setProperty('--vh', `${window.innerHeight}px`);
        window.addEventListener('resize', updateViewportVariable);
    }
}

/**
 * Update the viewport height variable "--vh".
 * This is called everytime there is a viewport change; e.g.: orientation change.
 */
function updateViewportVariable(): void {
    document.documentElement.style.setProperty('--vh', `${window.innerHeight}px`);
}

/**
 * Lifecycle hook after initial render.
 * Sets up variables from meta tags from config such satellite name, etc.
 */
onMounted(async (): Promise<void> => {
    try {
        await configStore.getConfig();
    } catch (error) {
        appStore.setErrorPage(500, true);
        notify.notifyError(error, null);
    }

    fixViewportHeight();

    isLoading.value = false;
});

onBeforeUnmount((): void => {
    window.removeEventListener('resize', updateViewportVariable);
});
</script>

<style lang="scss">
    @import 'static/styles/variables';

    * {
        margin: 0;
        padding: 0;
    }

    html {
        overflow: hidden;
        font-size: 14px;
    }

    body {
        margin: 0 !important;
        height: var(--vh, 100vh);
        zoom: 100%;
        overflow: hidden;
    }

    img,
    a {
        -webkit-user-drag: none;
    }

    #app {
        height: 100%;
    }

    @font-face {
        font-family: 'font_regular';
        font-style: normal;
        font-weight: 400;
        font-display: swap;
        src:
            local(''),
            url('@fontsource-variable/inter/files/inter-latin-standard-normal.woff2') format('woff2');
    }

    @font-face {
        font-family: 'font_medium';
        font-style: normal;
        font-weight: 600;
        font-display: swap;
        src:
            local(''),
            url('@fontsource-variable/inter/files/inter-latin-standard-normal.woff2') format('woff2');
    }

    @font-face {
        font-family: 'font_bold';
        font-style: normal;
        font-weight: 800;
        font-display: swap;
        src:
            local(''),
            url('@fontsource-variable/inter/files/inter-latin-standard-normal.woff2') format('woff2');
    }

    a {
        text-decoration: none;
        outline: none;
        cursor: pointer;
    }

    input,
    textarea {
        font-family: inherit;
        border: 1px solid rgb(56 75 101 / 40%);
        color: #354049;
        caret-color: #2683ff;
    }

    ::-webkit-scrollbar {
        width: 4px;
        height: 4px;
    }

    ::-webkit-scrollbar-track {
        box-shadow: inset 0 0 5px #fff;
    }

    ::-webkit-scrollbar-thumb {
        background: #afb7c1;
        border-radius: 6px;
        height: 5px;
    }

    ::-webkit-scrollbar-corner {
        background-color: transparent;
    }
</style>
