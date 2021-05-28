// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * ChartData class holds info for ChartData entity.
 */
export class ChartData {
    public labels: string[];
    public datasets: DataSets[] = [];

    public constructor(labels: string[], backgroundColor: string, borderColor: string, borderWidth: number, data: number[]) {
        this.labels = labels;

        for (let i = 0; i < this.labels.length; i++) {
            this.datasets[i] = new DataSets(backgroundColor, borderColor, borderWidth, data);
        }
    }
}

/**
 * DiskStatChartData class holds info for Disk Stat Chart.
 */
export class DiskStatChartData {
    public constructor(
        public datasets: DiskStatDataSet[] = [new DiskStatDataSet()],
    ) {}
}

/**
 * DiskStatDataSet describes all required data for disk stat chart dataset.
 */
export class DiskStatDataSet {
    public constructor(
        public label: string = '',
        public backgroundColor: string[] = ['#D6D6D6', '#0059D0', '#8FA7C6'],
        public data: number[] = [],
    ) {}
}

/**
 * DataSets class holds info for chart's DataSets entity.
 */
class DataSets {
    public backgroundColor: string;
    public borderColor: string;
    public borderWidth: number;
    public data: number[];

    public constructor(backgroundColor: string, borderColor: string, borderWidth: number, data: number[]) {
        this.backgroundColor = backgroundColor;
        this.borderColor = borderColor;
        this.borderWidth = borderWidth;
        this.data = data;
    }
}
