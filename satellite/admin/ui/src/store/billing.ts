// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';

import { UUID } from '@/types/common';

export const useBillingStore = defineStore('billing', () => {
    function getUsageReportLink(
        userID: UUID,
        since: Date,
        before: Date,
        projectSummary: boolean,
    ): string {
        let url = `/api/v1/users/${userID}/usage-report`;
        url += `?since=${since.toISOString()}&before=${before.toISOString()}`;
        if (projectSummary) url += '&projectSummary=true';
        return url;
    }

    return { getUsageReportLink };
});
