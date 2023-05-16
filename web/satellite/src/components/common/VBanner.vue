// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div
        v-if="isShown && bannerWidth > 0"
        class="notification-wrap"
        :class="{ warning: severity === 'warning', critical: severity === 'critical' }"
        @click="onClick"
    >
        <InfoIcon class="notification-wrap__icon" />
        <div class="notification-wrap__text">
            <slot name="text" />
        </div>
        <CloseIcon class="notification-wrap__close" @click="closeClicked" />
    </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue';

import InfoIcon from '@/../static/images/notifications/info.svg';
import CloseIcon from '@/../static/images/notifications/closeSmall.svg';

const props = withDefaults(defineProps<{
    severity?: 'info' | 'warning' | 'critical';
    onClick?: () => void;
    onClose?: () => void;
    dashboardRef: HTMLElement;
}>(), {
    severity: 'info',
    onClick: () => () => {},
    onClose: () => () => {},
});

const isShown = ref<boolean>(true);
const bannerWidth = ref<number>(0);
const resizeObserver = ref<ResizeObserver>();

function closeClicked(): void {
    isShown.value = false;
    if (props.onClose) {
        props.onClose();
    }
}

function onBannerResize(): void {
    bannerWidth.value = props.dashboardRef.offsetWidth;
}

function setResizable(): void {
    resizeObserver.value?.observe(props.dashboardRef);
}

onMounted((): void => {
    resizeObserver.value = new ResizeObserver(onBannerResize);

    if (props.dashboardRef) {
        setResizable();
        onBannerResize();
    }
});

onUnmounted((): void => {
    resizeObserver.value?.unobserve(props.dashboardRef);
});

watch(() => props.dashboardRef, () => {
    setResizable();
    onBannerResize();
});
</script>

<style scoped lang="scss">
.notification-wrap {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 1.375rem;
    font-family: 'font_regular', sans-serif;
    background-color: var(--c-light-blue-1);
    border: 1px solid var(--c-light-blue-2);
    border-radius: 10px;
    box-shadow: 0 7px 20px rgba(0 0 0 / 15%);

    @media screen and (width <= 450px) {
        flex-direction: column;
        align-items: flex-start;
        row-gap: 10px;
        position: relative;
    }

    &__icon {
        flex-shrink: 0;
        margin-right: 1.375rem;

        :deep(path) {
            fill: var(--c-blue-4);
        }
    }

    &.warning {
        background-color: var(--c-yellow-1);
        border: 1px solid var(--c-yellow-2);

        :deep(.icon-path) {
            fill: var(--c-yellow-3) !important;
        }
    }

    &.critical {
        background-color: var(--c-pink-1);
        border: 1px solid var(--c-pink-2);

        :deep(.icon-path) {
            fill: var(--c-pink-4);
        }
    }

    &__text {
        width: 100%;
        text-align: left;
        word-break: normal;
        font-size: 1rem;
        line-height: 1.625rem;
        display: flex;
        align-items: center;
        justify-content: space-between;
        column-gap: 10px;
    }

    &__close {
        width: 15px;
        height: 15px;
        margin-left: 2.375rem;
        cursor: pointer;
        flex-shrink: 0;

        @media screen and (width <= 450px) {
            position: absolute;
            right: 20px;
            top: 20px;
        }
    }
}

.bold {
    font-family: 'font_bold', sans-serif;
}

.medium {
    font-family: 'font_medium', sans-serif;
}

.link {
    color: black;
    text-decoration: underline !important;
    cursor: pointer;
}
</style>
