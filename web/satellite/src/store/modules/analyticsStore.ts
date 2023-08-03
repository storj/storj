// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';

import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';

export const useAnalyticsStore = defineStore('analytics', () => {
    const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

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
        eventTriggered,
        errorEventTriggered,
        linkEventTriggered,
        pageVisit,
    };
});
