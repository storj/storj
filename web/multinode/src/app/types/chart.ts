// Copyright (C) 2021 Storj Labs, Inc.
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

/**
 * StylingConstants holds tooltip styling constants
 */
class StylingConstants {
    public static tooltipOpacity = '1';
    public static tooltipPosition = 'absolute';
    public static pointWidth = '10px';
    public static pointHeight = '10px';
    public static borderRadius = '20px';
}

/**
 * Styling holds tooltip's styling configuration
 */
class Styling {
    public constructor(
        public tooltipModel: any,
        public element: HTMLElement,
        public topPosition: number,
        public leftPosition: number,
        public chartPosition: ClientRect,
    ) {}
}

/**
 * TooltipParams holds tooltip's configuration
 */
export class TooltipParams {
    public constructor(
        public tooltipModel: any,
        public chartId: string,
        public tooltipId: string,
        public pointId: string,
        public markUp: string,
        public tooltipTop: number,
        public tooltipLeft: number,
        public pointTop: number,
        public pointLeft: number,
        public color: string,
    ) {}
}

/**
 * Tooltip provides custom tooltip rendering
 */
export class Tooltip {
    public static custom(params: TooltipParams): void {
        const chart = document.getElementById(params.chartId);

        if (!chart) {
            return;
        }

        const tooltip: HTMLElement = Tooltip.createTooltip(params.tooltipId);
        const point: HTMLElement = Tooltip.createPoint(params.pointId);

        if (!params.tooltipModel.opacity) {
            Tooltip.remove(tooltip, point);

            return;
        }

        if (params.tooltipModel.body) {
            Tooltip.render(tooltip, params.markUp);
        }

        const position = chart.getBoundingClientRect();

        const tooltipStyling = new Styling(params.tooltipModel, tooltip, params.tooltipTop, params.tooltipLeft, position);

        Tooltip.elemStyling(tooltipStyling);

        const pointStyling = new Styling(params.tooltipModel, point, params.pointTop, params.pointLeft, position);

        Tooltip.elemStyling(pointStyling);

        Tooltip.pointStyling(point, params.color);
    }

    private static createTooltip(id: string): HTMLElement {
        let tooltipEl = document.getElementById(id);

        if (!tooltipEl) {
            tooltipEl = document.createElement('div');
            tooltipEl.id = id;
            document.body.appendChild(tooltipEl);
        }

        return tooltipEl;
    }

    private static createPoint(id: string): HTMLElement {
        let tooltipPoint = document.getElementById(id);

        if (!tooltipPoint) {
            tooltipPoint = document.createElement('div');
            tooltipPoint.id = id;
            document.body.appendChild(tooltipPoint);
        }

        return tooltipPoint;
    }

    private static remove(tooltipEl: HTMLElement, tooltipPoint: HTMLElement) {
        document.body.removeChild(tooltipEl);
        document.body.removeChild(tooltipPoint);
    }

    private static render(tooltip: HTMLElement, markUp: string) {
        tooltip.innerHTML = markUp;
    }

    private static elemStyling(elemStyling: Styling) {
        elemStyling.element.style.opacity = StylingConstants.tooltipOpacity;
        elemStyling.element.style.position = StylingConstants.tooltipPosition;
        elemStyling.element.style.left = `${elemStyling.chartPosition.left + elemStyling.tooltipModel.caretX - elemStyling.leftPosition}px`;
        elemStyling.element.style.top = `${elemStyling.chartPosition.top + window.pageYOffset + elemStyling.tooltipModel.caretY - elemStyling.topPosition}px`;
    }

    private static pointStyling(point: HTMLElement, color: string) {
        point.style.width = StylingConstants.pointWidth;
        point.style.height = StylingConstants.pointHeight;
        point.style.backgroundColor = color;
        point.style.borderRadius = StylingConstants.borderRadius;
    }
}
