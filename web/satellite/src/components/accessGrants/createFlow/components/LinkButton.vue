// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <a
        class="link-button"
        :href="link"
        target="_blank"
        rel="noopener noreferrer"
        @click="trackPageVisit"
    >
        <LearnIcon v-if="withIcon" class="link-button__icon" />
        <p class="link-button__label">{{ label }}</p>
    </a>
</template>

<script setup lang="ts">
import { AnalyticsHttpApi } from '@/api/analytics';

import LearnIcon from '@/../static/images/accessGrants/newCreateFlow/learn.svg';

const props = withDefaults(defineProps<{
    label: string;
    link: string;
    withIcon?: boolean;
}>(), {
    withIcon: false,
});

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

/**
 * Sends "trackPageVisit" event to segment.
 */
function trackPageVisit(): void {
    analytics.pageVisit(props.link);
}
</script>

<style scoped lang="scss">
.link-button {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 100%;
    height: 48px;
    background: #fff;
    border: 1px solid #d8dee3;
    border-radius: 8px;

    &__icon {
        margin-right: 8px;
    }

    &__label {
        font-family: 'font_medium', sans-serif;
        font-size: 14px;
        line-height: 24px;
        letter-spacing: -0.02em;
        color: #56606d;
    }

    &:hover {
        border-color: #2683ff;
        background-color: #2683ff;

        p {
            color: #fff;
        }

        :deep(svg path) {
            fill: #fff;
        }
    }

    &:focus {
        outline: 2px solid #376fff;
    }
}
</style>
