// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { formatBytes } from '@/utils/converter';

class BandwidthChartItem {
    public from: Date;
    public to: Date;
    public egress: Egress;
    public ingress: Ingress;
    public summary: number = 0;

    public constructor(data: FetchedBandwidthChartData) {
        this.from = new Date(data.from);
        this.to = new Date(data.to);
        this.egress = data.egress;
        this.ingress = data.ingress;

        this.getSum();
    }

    public static emptyWithDate(date: number): BandwidthChartItem {
        const now = new Date();
        now.setDate(date);

        const data: FetchedBandwidthChartData = {
            from:  now.toUTCString(),
            to: now.toUTCString(),
            egress: {
                audit: 0,
                repair: 0,
                usage: 0,
            },
            ingress: {
                repair: 0,
                usage: 0,
            },
        };

        return new BandwidthChartItem(data);
    }

    private getSum(): void {
        this.summary += this.egress.audit + this.egress.repair + this.egress.usage +
            this.ingress.repair + this.ingress.usage;
    }

    getLabels(): any {
        return {
            normalEgress: formatBytes(this.egress.usage),
            normalIngress: formatBytes(this.ingress.usage),
            repairIngress: formatBytes(this.ingress.repair),
            repairEgress: formatBytes(this.egress.repair),
            auditEgress: formatBytes(this.egress.audit),
            date: this.from.toUTCString(),
        };
    }
}

class StorageUsageChartItem {
    public atRestTotal: number;
    public timestamp: Date;

    public constructor(atRestTotal: number, timestamp: Date) {
        this.atRestTotal = atRestTotal;
        this.timestamp = timestamp;
    }

    public static fromFetchedData(data: FetchedStorageChartData): StorageUsageChartItem {
        return new StorageUsageChartItem(data.atRestTotal, new Date(data.timestamp));
    }

    public static emptyWithDate(date: number): StorageUsageChartItem {
        const now = new Date();
        let timestamp = new Date(now.getUTCFullYear(), now.getUTCMonth());

        return new StorageUsageChartItem(0, timestamp);
    }

    public getLabels(): object {
        return {
            atRestTotal: formatBytes(this.atRestTotal),
            timestamp: this.timestamp.toUTCString(),
        };
    }
}

export class ChartFormatter {
    // createBandwidthChartItems creates new BandwidthChartItem array from start of the month till today
    // and fills it with fetched data if there is data for given date, or creates empty item otherwise
    public static createBandwidthChartItems(fetchedData: FetchedBandwidthChartData[]): BandwidthChartItem[] {
        const bandwidthChartData: BandwidthChartItem[] = new Array(new Date().getDate());

        outer:
        for (let i = 0; i < bandwidthChartData.length; i++) {
            const date = i + 1;

            for (let j = 0; j < fetchedData.length; j++) {
                const fetched = fetchedData[j];

                if (new Date(fetched.from).getDate() === date) {
                    bandwidthChartData[i] = new BandwidthChartItem(fetched);
                    continue outer;
                }
            }

            bandwidthChartData[i] = BandwidthChartItem.emptyWithDate(date);
        }

        return bandwidthChartData;
    }

     public static createStorageUsageChartItems(fetchedData: FetchedStorageChartData[]): StorageUsageChartItem[] {
        const storageChartData: StorageUsageChartItem[] = new Array( new Date().getDate());

        if (storageChartData.length === 0 || fetchedData.length === 0) {
            return storageChartData;
        }

        outer:
        for (let i = 0; i < storageChartData.length; i++ ) {
            const date = i + 1;

            for (let j = 0; j < fetchedData.length; j++) {
                const storageUsage = StorageUsageChartItem.fromFetchedData(fetchedData[j]);

                if (storageUsage.timestamp.getDate() === date) {
                    storageChartData[i] = storageUsage;
                    continue outer;
                }

                storageChartData[i] = StorageUsageChartItem.emptyWithDate(date);
            }
        }

        return storageChartData;
    }
}
