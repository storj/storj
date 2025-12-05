// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { computed } from 'vue';

import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { JoinCunoFSBetaForm, ObjectMountConsultationForm, UserFeedbackForm } from '@/types/analytics';
import { useConfigStore } from '@/store/modules/configStore';

export const useAnalyticsStore = defineStore('analytics', () => {
    const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    const configStore = useConfigStore();
    const csrfToken = computed<string>(() => configStore.state.config.csrfToken);

    async function ensureEventTriggered(eventName: AnalyticsEvent, props?: { [p: string]: string }): Promise<void> {
        await analytics.ensureEventTriggered(eventName, csrfToken.value, props);
    }

    async function joinCunoFSBeta(data: JoinCunoFSBetaForm): Promise<void> {
        await analytics.joinCunoFSBeta(data, csrfToken.value);
    }

    async function joinPlacementWaitlist(storageNeeds: string, placementID: number): Promise<void> {
        await analytics.joinPlacementWaitlist(storageNeeds, placementID, csrfToken.value);
    }

    async function sendUserFeedback(data: UserFeedbackForm): Promise<void> {
        await analytics.sendUserFeedback(data, csrfToken.value);
    }

    async function requestObjectMountConsultation(data: ObjectMountConsultationForm): Promise<void> {
        await analytics.requestObjectMountConsultation(data, csrfToken.value);
    }

    function eventTriggered(eventName: AnalyticsEvent, props?: { [p: string]: string }): void {
        analytics.eventTriggered(eventName, csrfToken.value, props).catch(_ => { });
    }

    function linkEventTriggered(eventName: AnalyticsEvent, link: string): void {
        analytics.linkEventTriggered(eventName, link, csrfToken.value).catch(_ => { });
    }

    function pageVisit(pagePath: string, source: string): void {
        analytics.pageVisit(pagePath, csrfToken.value).catch(_ => { });

        let url: string;
        if (pagePath.includes('http')) {
            // external link
            url = pagePath;
        } else {
            url = window.location.protocol + '//' + window.location.host + pagePath;
            if (window.location.search) {
                // remove sensitive query params
                const avoidKeys = ['token', 'email', 'inviter_email', 'projectID'];
                const filteredParams: string[] = [];
                const params = window.location.search.replace('?', '').split('&');
                for (const param of params) {
                    if (!avoidKeys.find((k) => param.includes(k))) {
                        filteredParams.push(param);
                    }
                }
                if (filteredParams.length > 0) url = url + '?' + filteredParams.join('&');
            }
        }

        analytics.pageView({ url, props: { source } });
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
        joinCunoFSBeta,
        joinPlacementWaitlist,
        requestObjectMountConsultation,
        sendUserFeedback,
    };
});
