// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="error-area d-flex flex-column justify-center">
        <div class="error-area__logo-wrapper d-flex justify-center">
            <v-img
                v-if="theme.global.current.value.dark"
                class="error-area__logo-wrapper__logo"
                :src="configStore.darkLogo"
                width="140"
                alt="Logo"
                @click="goToHomepage"
            />
            <v-img
                v-else
                class="error-area__logo-wrapper__logo"
                :src="configStore.logo"
                width="140"
                alt="Logo"
                @click="goToHomepage"
            />
        </div>
        <div class="d-flex flex-column align-center text-center">
            <h2 class="text-h2 font-weight-bold mb-4">{{ statusCode }}</h2>
            <h5 class="text-h5 mb-6">{{ message }}</h5>
            <v-btn
                @click="onButtonClick"
            >
                {{ 'Go ' + (isFatal ? 'to homepage' : 'back') }}
            </v-btn>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { VBtn, VImg } from 'vuetify/components';
import { useTheme } from 'vuetify';

import { useAppStore } from '@/store/modules/appStore';
import { useConfigStore } from '@/store/modules/configStore';

const appStore = useAppStore();
const configStore = useConfigStore();
const router = useRouter();
const route = useRoute();
const theme = useTheme();

const messages = new Map<number, string>([
    [404, 'Oops, page not found.'],
    [500, 'Internal server error.'],
]);

/**
 * Retrieves the error's status code from the store.
 */
const statusCode = computed((): number => {
    return appStore.state.error.statusCode;
});

/**
 * Retrieves the message corresponding to the error's status code.
 */
const message = computed((): string => {
    return messages.get(statusCode.value) || 'An error occurred.';
});

/**
 * Indicates whether the error is unrecoverable.
 */
const isFatal = computed((): boolean => {
    return appStore.state.error.fatal;
});

/**
 * Navigates to the homepage.
 */
function goToHomepage(): void {
    window.location.href = configStore.homepageUrl;
}

/**
 * Navigates to the homepage if fatal or the previous route otherwise.
 */
function onButtonClick(): void {
    if (isFatal.value) {
        goToHomepage();
        return;
    }
    if (!route.redirectedFrom) {
        router.replace('/');
        return;
    }
    router.back();
}

/**
 * Lifecycle hook after initial render. Sets page title.
 */
onMounted(() => {
    const satName = configStore.state.config.satelliteName;
    document.title = statusCode.value.toString() + (satName ? ' | ' + satName : '');
});
</script>

<style scoped lang="scss">
    .error-area {
        position: fixed;
        inset: 0;
        padding: 52px 24px;
        box-sizing: border-box;
        overflow-y: auto;
        background: rgb(var(--v-theme-background));

        &:before {
            content: '';
            position: fixed;
            inset: 0;
            background: url('../assets/icon-world.svg') no-repeat center;
            background-size: auto 75%;
            z-index: -1;
            opacity: 0.1;
        }

        &__logo-wrapper {
            height: 42px;
            position: absolute;
            top: 52px;
            left: 0;
            right: 0;
            margin-bottom: 52px;

            &__logo {
                height: 100%;
                width: auto;
                cursor: pointer;
            }
        }
    }

    @media screen and (height <= 500px) {

        .error-area {
            justify-content: flex-start;

            &__logo-wrapper {
                position: unset;
            }
        }
    }
</style>
