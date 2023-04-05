// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="beta-banner">
        <p class="beta-banner__message">
            Thanks for testing the {{ config.satelliteName }} Beta satellite | Data may be deleted during this beta | Submit testing feedback
            <a class="beta-banner__message__link" :href="config.betaSatelliteFeedbackURL" target="_blank" rel="noopener noreferrer">here</a>
            | Request support
            <a class="beta-banner__message__link" :href="config.betaSatelliteSupportURL" target="_blank" rel="noopener noreferrer">here</a>
        </p>
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { useAppStore } from '@/store/modules/appStore';
import { FrontendConfig } from '@/types/config';

const appStore = useAppStore();

/**
 * Returns the frontend config.
 */
const config = computed((): FrontendConfig => {
    return appStore.state.config;
});
</script>

<style scoped lang="scss">
    .beta-banner {
        width: 100%;
        box-sizing: border-box;
        padding: 5px 30px;
        display: flex;
        align-items: center;
        justify-content: space-between;
        font-family: 'font_regular', sans-serif;
        background-color: red;

        &__message {
            font-weight: normal;
            font-size: 14px;
            line-height: 16px;
            color: #fff;

            &__link {
                font-size: 14px;
                line-height: 16px;
                color: #fff;
                text-decoration: underline;

                &:hover {
                    text-decoration: none;
                }
            }
        }
    }
</style>
