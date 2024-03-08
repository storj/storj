// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { ref } from 'vue';

import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';

type Plausible = (event: 'pageview', data: { u: string; props?: { satellite: string } }) => void;

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

    function eventTriggered(eventName: AnalyticsEvent, props?: {[p: string]: string}): void {
        analytics.eventTriggered(eventName, props).catch(_ => {});
    }

    function linkEventTriggered(eventName: AnalyticsEvent, link: string): void {
        analytics.linkEventTriggered(eventName, link).catch(_ => {});
    }

    function pageVisit(pageName: string): void {
        analytics.pageVisit(pageName).catch(_ => {});
    }

    function errorEventTriggered(source: AnalyticsErrorEventSource): void {
        analytics.errorEventTriggered(source).catch(_ => {});
    }

    return {
        loadPlausible,
        eventTriggered,
        errorEventTriggered,
        linkEventTriggered,
        pageVisit,
    };
});
