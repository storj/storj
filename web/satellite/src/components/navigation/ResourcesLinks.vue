// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <a
            class="dropdown-item"
            href="https://docs.storj.io/"
            target="_blank"
            rel="noopener noreferrer"
            @click.prevent="trackViewDocsEvent('https://docs.storj.io/')"
        >
            <DocsIcon class="dropdown-item__icon" />
            <div class="dropdown-item__text">
                <h2 class="dropdown-item__text__title">Docs</h2>
                <p class="dropdown-item__text__label">Documentation for Storj</p>
            </div>
        </a>
        <a
            class="dropdown-item"
            href="https://forum.storj.io/"
            target="_blank"
            rel="noopener noreferrer"
            @click.prevent="trackViewForumEvent('https://forum.storj.io/')"
        >
            <ForumIcon class="dropdown-item__icon" />
            <div class="dropdown-item__text">
                <h2 class="dropdown-item__text__title">Forum</h2>
                <p class="dropdown-item__text__label">Join our global community</p>
            </div>
        </a>
        <a
            class="dropdown-item"
            href="https://supportdcs.storj.io/hc/en-us"
            target="_blank"
            rel="noopener noreferrer"
            @click.prevent="trackViewSupportEvent('https://supportdcs.storj.io/hc/en-us')"
        >
            <SupportIcon class="dropdown-item__icon" />
            <div class="dropdown-item__text">
                <h2 class="dropdown-item__text__title">Support</h2>
                <p class="dropdown-item__text__label">Get technical support</p>
            </div>
        </a>
    </div>
</template>

<script setup lang="ts">
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';

import DocsIcon from '@/../static/images/navigation/docs.svg';
import ForumIcon from '@/../static/images/navigation/forum.svg';
import SupportIcon from '@/../static/images/navigation/support.svg';

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

/**
 * Sends "View Docs" event to segment and opens link.
 */
function trackViewDocsEvent(link: string): void {
    analytics.pageVisit(link);
    analytics.eventTriggered(AnalyticsEvent.VIEW_DOCS_CLICKED);
    window.open(link);
}

/**
 * Sends "View Forum" event to segment and opens link.
 */
function trackViewForumEvent(link: string): void {
    analytics.pageVisit(link);
    analytics.eventTriggered(AnalyticsEvent.VIEW_FORUM_CLICKED);
    window.open(link);
}

/**
 * Sends "View Support" event to segment and opens link.
 */
function trackViewSupportEvent(link: string): void {
    analytics.pageVisit(link);
    analytics.eventTriggered(AnalyticsEvent.VIEW_SUPPORT_CLICKED);
    window.open(link);
}
</script>

<style scoped lang="scss">
    .dropdown-item:focus {
        background-color: #f5f6fa;
        outline: auto;
        outline-color: var(--c-blue-3);
    }
</style>
