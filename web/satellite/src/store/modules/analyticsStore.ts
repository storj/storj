// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';

import { AnalyticsHttpApi } from '@/api/analytics';

export const useAnalyticsStore = defineStore('analytics', () => {
    const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    function eventTriggered(eventName: string): void {
        analytics.eventTriggered(eventName).then(r => {}).catch();
    }

    function linkEventTriggered(eventName: string, link: string): void {
        analytics.linkEventTriggered(eventName, link).then(r => {}).catch();
    }

    function pageVisited(pageName: string): void {
        analytics.pageVisit(pageName).then(r => {}).catch();
    }

    return {
        eventTriggered,
        linkEventTriggered,
        pageVisited,
    };
});
