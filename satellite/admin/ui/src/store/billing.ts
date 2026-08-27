// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';

import {
    TokenTransaction,
    UserManagementHttpApiV1,
    UserTokenBalance,
} from '@/api/client.gen';
import { UUID } from '@/types/common';

export const useBillingStore = defineStore('billing', () => {
    const userApi = new UserManagementHttpApiV1();

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

    async function getTokenBalance(userID: string): Promise<UserTokenBalance> {
        return userApi.getUserTokenBalance(userID);
    }

    async function getTokenTransactions(userID: string): Promise<TokenTransaction[]> {
        const response = await userApi.getUserTokenTransactions(userID);
        return response.transactions || [];
    }

    return { getUsageReportLink, getTokenBalance, getTokenTransactions };
});
