// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { formatBytes } from '@/utils/converter';

class EmptyBandwidthChartDataItem implements BandwidthChartData {
    public From: string;
    public To: string;
    public egress: Egress;
    public ingress: Ingress;
    public summary: number;

    public constructor(i) {
        let date = new Date();
        date.setDate(i);
        this.From = date.toLocaleDateString();
        this.To = '';

        this.egress = {
            audit: 0,
            repair: 0,
            usage: 0
        } as Egress;

        this.ingress = {
            repair: 0,
            usage: 0
        } as Ingress;

        this.summary = 0;
    }

    getLabels(): any {
        return {
            normalEgress: formatBytes(this.egress.usage),
            normalIngress: formatBytes(this.ingress.usage),
            repairIngress: formatBytes(this.ingress.repair),
            repairEgress: formatBytes(this.egress.repair),
            auditEgress: formatBytes(this.egress.audit),
            date: this.From
        };
    }
}

class BandwidthChartDataItem implements BandwidthChartData {
    public From: string;
    public To: string;
    public egress: Egress;
    public ingress: Ingress;
    public summary: number = 0;

    public constructor(actualData: FetchedBandwidthChartData) {
        this.From = actualData.From;
        this.To = actualData.To;
        this.egress = actualData.egress;
        this.ingress = actualData.ingress;

        this.getSum();
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
            date: new Date().toLocaleString()
        };
    }
}

export class BandwidthChartDataFormatter {
    private readonly data: BandwidthChartData[] = [];

    public constructor(banChartData: FetchedBandwidthChartData[]) {
        this.data = this.formatActualData(banChartData);

        this.fillChartData();
    }

    public getFormattedData(): BandwidthChartData[] {
        return this.data;
    }

    private getMonthDays(): number {
        return new Date().getDate();
    }

    private formatActualData(banChartData: FetchedBandwidthChartData[]): BandwidthChartData[] {
        return banChartData.map(element => {
            return new BandwidthChartDataItem(element);
        });
    }

    private fillChartData(): void {
        const numbersOfDays = this.getMonthDays();

        if (this.data.length < numbersOfDays) {
            for (let i = numbersOfDays - this.data.length; i > 0; i--) {
                this.data.unshift(new EmptyBandwidthChartDataItem(i));
            }
        }
    }
}
