// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { computed } from 'vue';

import { AnalyticsHttpApi } from '@/api/analytics';
import type { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import type { UserFeedbackForm } from '@/types/analytics';
import { useConfigStore } from '@/store/modules/configStore';

export const useAnalyticsStore = defineStore('analytics', () => {
    const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    const configStore = useConfigStore();
    const csrfToken = computed<string>(() => configStore.state.config.csrfToken);

    async function ensureEventTriggered(eventName: AnalyticsEvent, props?: { [p: string]: string }): Promise<void> {
        await analytics.ensureEventTriggered(eventName, csrfToken.value, props);
    }

    async function joinPlacementWaitlist(storageNeeds: string, placementID: number): Promise<void> {
        await analytics.joinPlacementWaitlist(storageNeeds, placementID, csrfToken.value);
    }

    async function sendUserFeedback(data: UserFeedbackForm): Promise<void> {
        await analytics.sendUserFeedback(data, csrfToken.value);
    }

    function eventTriggered(eventName: AnalyticsEvent, props?: { [p: string]: string }): void {
        analytics.eventTriggered(eventName, csrfToken.value, props).catch(_ => { });
    }

    function linkEventTriggered(eventName: AnalyticsEvent, link: string): void {
        analytics.linkEventTriggered(eventName, link, csrfToken.value).catch(_ => { });
    }

    function pageVisit(pagePath: string): void {
        analytics.pageVisit(pagePath, csrfToken.value).catch(_ => { });
    }

    function errorEventTriggered(source: AnalyticsErrorEventSource, requestID: string | null = null, statusCode?: number): void {
        analytics.errorEventTriggered(source, csrfToken.value, requestID, statusCode).catch(_ => { });
    }

    return {
        ensureEventTriggered,
        eventTriggered,
        errorEventTriggered,
        linkEventTriggered,
        pageVisit,
        joinPlacementWaitlist,
        sendUserFeedback,
    };
});
