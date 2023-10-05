// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="error-area">
        <div class="error-area__logo-wrapper">
            <Logo class="error-area__logo-wrapper__logo" @click="goToHomepage" />
        </div>
        <div class="error-area__container">
            <h1 class="error-area__container__title">{{ statusCode }}</h1>
            <h2 class="error-area__container__subtitle">{{ message }}</h2>
            <VButton
                class="error-area__container__button"
                :label="'Go ' + (isFatal ? 'to homepage' : 'back')"
                width="auto"
                height="auto"
                border-radius="6px"
                :on-press="onButtonClick"
            />
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue';
import { useRouter } from 'vue-router';

import { useAppStore } from '@/store/modules/appStore';
import { useConfigStore } from '@/store/modules/configStore';

import VButton from '@/components/common/VButton.vue';

import Logo from '@/../static/images/logo.svg';

const router = useRouter();
const configStore = useConfigStore();
const appStore = useAppStore();

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
    window.location.href = configStore.state.config.homepageURL || 'https://www.storj.io';
}

/**
 * Navigates to the homepage if fatal or the previous route otherwise.
 */
function onButtonClick(): void {
    if (isFatal.value) {
        goToHomepage();
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
        display: flex;
        flex-direction: column;
        justify-content: center;
        overflow-y: auto;

        &:before {
            content: '';
            position: fixed;
            inset: 0;
            background: url('/static/static/images/errors/world.svg') no-repeat center;
            background-size: auto 75%;
            z-index: -1;
            opacity: 0.15;
        }

        &__logo-wrapper {
            height: 30.89px;
            display: flex;
            justify-content: center;
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

        &__container {
            display: flex;
            flex-direction: column;
            align-items: center;
            font-family: 'font_bold', sans-serif;
            text-align: center;

            &__title {
                font-size: 90px;
                line-height: 90px;
            }

            &__subtitle {
                margin-top: 20px;
                font-size: 28px;
            }

            &__button {
                margin-top: 32px;
                padding: 16px 37.5px;

                :deep(.label) {
                    font-family: 'font_bold', sans-serif;
                    line-height: 20px;
                }
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
