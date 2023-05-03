// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="os">
        <div class="os__tabs">
            <p
                class="os__tabs__choice"
                :class="{active: isWindows && !isInstallStep, 'active-install-step': isWindows && isInstallStep}"
                aria-roledescription="windows"
                @click="setTab(OnboardingOS.WINDOWS)"
            >
                Windows
            </p>
            <p
                class="os__tabs__choice"
                :class="{active: isLinux && !isInstallStep, 'active-install-step': isLinux && isInstallStep}"
                aria-roledescription="linux"
                @click="setTab(OnboardingOS.LINUX)"
            >
                Linux
            </p>
            <p
                class="os__tabs__choice"
                :class="{active: isMacOS && !isInstallStep, 'active-install-step': isMacOS && isInstallStep}"
                aria-roledescription="macos"
                @click="setTab(OnboardingOS.MAC)"
            >
                macOS
            </p>
        </div>
        <slot v-if="isWindows" :name="OnboardingOS.WINDOWS" />
        <slot v-if="isLinux" :name="OnboardingOS.LINUX" />
        <slot v-if="isMacOS" :name="OnboardingOS.MAC" />
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';

import { OnboardingOS } from '@/types/common';
import { useAppStore } from '@/store/modules/appStore';

const appStore = useAppStore();

const props = withDefaults(defineProps<{
    isInstallStep?: boolean;
}>(), {
    isInstallStep: false,
});

const osSelected = ref<OnboardingOS>(OnboardingOS.WINDOWS);

/**
 * Returns selected os from store.
 */
const storedOsSelected = computed((): OnboardingOS | null => {
    return appStore.state.onbSelectedOs;
});

/**
 * Indicates if mac tab is selected.
 */
const isMacOS = computed((): boolean => {
    return osSelected.value === OnboardingOS.MAC;
});

/**
 * Indicates if linux tab is selected.
 */
const isLinux = computed((): boolean => {
    return osSelected.value === OnboardingOS.LINUX;
});

/**
 * Indicates if windows tab is selected.
 */
const isWindows = computed((): boolean => {
    return osSelected.value === OnboardingOS.WINDOWS;
});

/**
 * Sets view to given state.
 */
function setTab(os: OnboardingOS): void {
    osSelected.value = os;
    appStore.setOnboardingOS(os);
}

/**
 * Lifecycle hook after initial render.
 * Checks user's OS.
 */
onMounted((): void => {
    if (storedOsSelected.value) {
        setTab(storedOsSelected.value);
        return;
    }

    switch (true) {
    case navigator.appVersion.indexOf('Mac') !== -1:
        setTab(OnboardingOS.MAC);
        return;
    case navigator.appVersion.indexOf('Linux') !== -1:
        setTab(OnboardingOS.LINUX);
        return;
    default:
        setTab(OnboardingOS.WINDOWS);
    }
});
</script>

<style scoped lang="scss">
    .os {
        font-family: 'font_regular', sans-serif;
        margin-top: 20px;
        width: 100%;

        &__tabs {
            display: inline-flex;
            align-items: center;
            border-radius: 0 6px 6px 0;
            border-top: 1px solid rgb(230 236 241);
            border-left: 1px solid rgb(230 236 241);
            border-right: 1px solid rgb(230 236 241);
            margin-bottom: -1px;

            &__choice {
                background-color: #f5f7f9;
                font-size: 14px;
                padding: 16px;
                color: #9daab6;
                cursor: pointer;
            }

            &__choice:first-child {
                border-right: 1px solid rgb(230 236 241);
            }

            &__choice:last-child {
                border-left: 1px solid rgb(230 236 241);
            }
        }
    }

    .active {
        background: rgb(24 48 85);
        color: #e6ecf1;
    }

    .active-install-step {
        background: #fff;
        color: #242a31;
    }
</style>
