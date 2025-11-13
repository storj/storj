// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div id="app">
        <div class="container">
            <SNOHeader />
            <div class="scrollable" @scroll="onScroll">
                <router-view />
                <SNOFooter />
            </div>
        </div>
        <LoadingScreen v-if="isLoading" />
    </div>
</template>

<script setup lang="ts">
import { computed, onBeforeMount } from 'vue';

import { useAppStore } from '@/app/store/modules/appStore';

import LoadingScreen from '@/app/components/LoadingScreen.vue';
import SNOFooter from '@/app/components/SNOFooter.vue';
import SNOHeader from '@/app/components/SNOHeader.vue';

const appStore = useAppStore();

const elementsIdsToRemoveOnScroll: string[] = [
    'bandwidth-tooltip',
    'bandwidth-tooltip-arrow',
    'bandwidth-tooltip-point',
    'disk-space-tooltip',
    'disk-space-tooltip-arrow',
    'disk-space-tooltip-point',
    'egress-tooltip',
    'egress-tooltip-arrow',
    'egress-tooltip-point',
    'ingress-tooltip',
    'ingress-tooltip-arrow',
    'ingress-tooltip-point',
];

const elementsClassesToRemoveOnScroll: string[] = [
    'info__message-box',
    'payout-period-calendar',
    'notification-popup-container',
];

const isLoading = computed<boolean>(() => {
    return appStore.state.isLoading;
});

function onScroll(): void {
    elementsIdsToRemoveOnScroll.forEach(id => {
        removeElementById(id);
    });

    elementsClassesToRemoveOnScroll.forEach(className => {
        removeElementByClass(className);
    });
}

function removeElementByClass(className: string): void {
    const element: HTMLElement | null = document.querySelector(`.${className}`);
    if (element) {
        element.remove();
    }
}

function removeElementById(id: string): void {
    const element: HTMLElement | null = document.getElementById(id);
    if (element) {
        element.remove();
    }
}

onBeforeMount(() => {
    document.body.classList.add('js-loading');
    window.onload = () => {
        document.body.classList.remove('js-loading');
    };
});
</script>

<style lang="scss">
    @import '../../static/styles/variables';

    ::-webkit-scrollbar {
        display: none;
        position: fixed;
        right: 0;
    }

    body {
        margin: 0 !important;
        position: relative;
        font-family: 'font_regular', sans-serif;
        overflow-y: hidden;
    }

    button {
        all: unset;
    }

    .container {
        background-color: var(--app-background-color);
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        height: auto;
    }

    .scrollable {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        padding-top: 89px;
        height: calc(100vh - 89px);
        width: 100vw;
        overflow-y: scroll;
    }

    .back-button :deep(path) {
        fill: var(--regular-icon-color) !important;
    }

    .js-loading *,
    .js-loading *:before,
    .js-loading *:after {
        animation-play-state: paused !important;
    }

    @font-face {
        font-display: swap;
        font-family: 'font_regular';
        src: url('../../static/fonts/font_regular.ttf');
    }

    @font-face {
        font-display: swap;
        font-family: 'font_medium';
        src: url('../../static/fonts/font_medium.ttf');
    }

    @font-face {
        font-display: swap;
        font-family: 'font_bold';
        src: url('../../static/fonts/font_bold.ttf');
    }
</style>
