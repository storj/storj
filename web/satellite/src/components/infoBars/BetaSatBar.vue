// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="beta-banner">
        <p class="beta-banner__message">
            Thanks for testing the {{ satelliteName }} Beta satellite | Data may be deleted during this beta | Submit testing feedback
            <a class="beta-banner__message__link" :href="betaFeedbackURL" target="_blank" rel="noopener noreferrer">here</a>
            | Request support
            <a class="beta-banner__message__link" :href="betaSupportURL" target="_blank" rel="noopener noreferrer">here</a>
        </p>
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { MetaUtils } from '@/utils/meta';

/**
 * Returns satellite name from store (config).
 */
const satelliteName = computed((): string => {
    return MetaUtils.getMetaContent('satellite-name');
});

/**
 * Returns feedback URL from config for beta satellites.
 */
const betaFeedbackURL = computed((): string => {
    return MetaUtils.getMetaContent('beta-satellite-feedback-url');
});

/**
 * Returns support URL from config for beta satellites.
 */
const betaSupportURL = computed((): string => {
    return MetaUtils.getMetaContent('beta-satellite-support-url');
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
