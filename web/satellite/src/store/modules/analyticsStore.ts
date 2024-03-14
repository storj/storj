// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { ref } from 'vue';

import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';

type Plausible = (event: 'pageview', data: { u: string; props: { source: string } }) => void;

export const useAnalyticsStore = defineStore('analytics', () => {
    const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    const plausible = ref<Plausible>();

    async function loadPlausible(data: {
        domain: string;
        scriptURL: string;
    }) {
        try {
            await new Promise((resolve, reject) => {
                const head = document.head || document.getElementsByTagName('head')[0];
                const script = document.createElement('script');
                script.type = 'text/javascript';
                script.src = data.scriptURL;
                script.setAttribute('data-domain', data.domain);

                head.appendChild(script);
                script.onload = resolve;
                script.onerror = reject;
            });
            plausible.value = window['plausible'];
        } catch (_) { /*empty*/ }
    }

    function eventTriggered(eventName: AnalyticsEvent, props?: Map<string, string>): void {
        analytics.eventTriggered(eventName, props).catch(_ => { });
    }

    function linkEventTriggered(eventName: AnalyticsEvent, link: string): void {
        analytics.linkEventTriggered(eventName, link).catch(_ => { });
    }

    function pageVisit(pagePath: string, source: string): void {
        analytics.pageVisit(pagePath).catch(_ => { });

        if (!plausible.value) {
            return;
        }

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

        plausible.value('pageview', { u: url, props: { source } });
    }

    function errorEventTriggered(source: AnalyticsErrorEventSource): void {
        analytics.errorEventTriggered(source).catch(_ => { });
    }

    return {
        loadPlausible,
        eventTriggered,
        errorEventTriggered,
        linkEventTriggered,
        pageVisit,
    };
});
