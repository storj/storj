// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';

import { ReportsHttpApiV1, UsageReportPage } from '@/api/client.gen';

export const useReportsStore = defineStore('reports', () => {
    const reportsApi = new ReportsHttpApiV1();

    async function getUsageReport(period: string, limit: number, cursor: string): Promise<UsageReportPage> {
        return reportsApi.getUsageReport(period, limit.toString(), cursor);
    }

    function getUsageReportCSVLink(period: string): string {
        return `/api/v1/reports/usage/csv?period=${encodeURIComponent(period)}`;
    }

    return {
        getUsageReport,
        getUsageReportCSVLink,
    };
});
