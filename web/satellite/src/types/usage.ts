// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { Size } from '@/utils/bytesSize';

/**
 * ProjectUsage sums usage for given period.
 */
export class ProjectUsage {
    public storage: Size;
    public egress: Size;
    public objectCount: number;
    public since: Date;
    public before: Date;

    public constructor(storage: number, egress: number, objectCount: number, since: Date, before: Date) {
        this.storage = new Size(storage, 4);
        this.egress = new Size(egress, 4);
        this.objectCount = objectCount;
        this.since = since;
        this.before = before;
    }
}

/**
 * Holds start and end dates.
 */
export class DateRange {
    public startDate: Date = new Date();
    public endDate: Date = new Date();

    public constructor(startDate: Date, endDate: Date) {
        this.startDate = startDate;
        this.endDate = endDate;
    }
}

/**
 * Exposes all project-usage-related functionality.
 */
export interface UsageApi {
    /**
     * Fetches usage.
     *
     * @returns ProjectUsage
     * @throws Error
     */
    get(projectId: string, since: Date, before: Date): Promise<ProjectUsage>;
}
