// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';

import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';

export const useAnalyticsStore = defineStore('analytics', () => {
    const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    function eventTriggered(eventName: AnalyticsEvent, props?: Map<string, string>): void {
        analytics.eventTriggered(eventName, props).catch(_ => { });
    }

    function linkEventTriggered(eventName: AnalyticsEvent, link: string): void {
        analytics.linkEventTriggered(eventName, link).catch(_ => { });
    }

    function pageVisit(pagePath: string, source: string): void {
        analytics.pageVisit(pagePath).catch(_ => { });

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

    function errorEventTriggered(source: AnalyticsErrorEventSource): void {
        analytics.errorEventTriggered(source).catch(_ => { });
    }

    return {
        eventTriggered,
        errorEventTriggered,
        linkEventTriggered,
        pageVisit,
    };
});
