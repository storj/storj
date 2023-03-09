// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { DataStamp } from '@/types/projects';
import { Size } from '@/utils/bytesSize';

/**
 * ChartData class holds info for ChartData entity.
 */
export class ChartData {
    public labels: string[];
    public datasets: DataSets[] = [];

    public constructor(
        labels: string[],
        backgroundColor: string,
        borderColor: string,
        pointBorderColor: string,
        data: number[],
        secondaryBackgroundColor?: string,
        secondaryBorderColor?: string,
        secondaryPointBorderColor?: string,
        secondaryData?: number[],
    ) {
        this.labels = labels;
        this.datasets[0] = new DataSets(backgroundColor, borderColor, pointBorderColor, data);

        if (secondaryData && secondaryBackgroundColor && secondaryBorderColor && secondaryPointBorderColor) {
            this.datasets[1] = new DataSets(secondaryBackgroundColor, secondaryBorderColor, secondaryPointBorderColor, secondaryData);
        }
    }
}

/**
 * DataSets class holds info for chart's DataSets entity.
 */
class DataSets {
    public constructor(
        public backgroundColor: string,
        public borderColor: string,
        public pointBorderColor: string,
        public data: number[],
        public borderWidth: number = 4,
        public pointHoverBackgroundColor: string = 'white',
        public pointHoverBorderWidth: number = 5,
    ) {}
}

/**
 * TooltipParams holds tooltip's configuration
 */
export class TooltipParams {
    public constructor(
        public tooltipModel: TooltipModel,
        public chartId: string,
        public tooltipId: string,
        public markUp: string,
        public tooltipTop: number,
        public tooltipLeft: number,
    ) {}
}

/**
 * StylingConstants holds tooltip styling constants
 */
class StylingConstants {
    public static tooltipOpacity = '1';
    public static tooltipPosition = 'absolute';
}

/**
 * Styling holds tooltip's styling configuration
 */
class Styling {
    public constructor(
        public tooltipModel: TooltipModel,
        public element: HTMLElement,
        public topPosition: number,
        public leftPosition: number,
        public chartPosition: ClientRect,
    ) {}
}

/**
 * Color is a color definition.
 */
export type Color = string

/**
 * TooltipItem contains datapoint information.
 */
export interface TooltipItem {
    // Label for the tooltip
    label: string,

    // Value for the tooltip
    value: string,

    // X Value of the tooltip
    // (deprecated) use `value` or `label` instead
    xLabel: number | string,

    // Y value of the tooltip
    // (deprecated) use `value` or `label` instead
    yLabel: number | string,

    // Index of the dataset the item comes from
    datasetIndex: number,

    // Index of this data item in the dataset
    index: number,

    // X position of matching point
    x: number,

    // Y position of matching point
    y: number
}

/**
 * TooltipModel contains parameters that can be used to render the tooltip.
 */
export interface TooltipModel {
    // The items that we are rendering in the tooltip. See Tooltip Item Interface section
    dataPoints: TooltipItem[],

    // Positioning
    xPadding: number,
    yPadding: number,
    xAlign: string,
    yAlign: string,

    // X and Y properties are the top left of the tooltip
    x: number,
    y: number,
    width: number,
    height: number,
    // Where the tooltip points to
    caretX: number,
    caretY: number,

    // Body
    // The body lines that need to be rendered
    // Each object contains 3 parameters
    // before: string[] // lines of text before the line with the color square
    // lines: string[], // lines of text to render as the main item with color square
    // after: string[], // lines of text to render after the main lines
    body: {before: string[]; lines: string[], after: string[]}[],
    // lines of text that appear after the title but before the body
    beforeBody: string[],
    // line of text that appear after the body and before the footer
    afterBody: string[],
    bodyFontColor: Color,
    _bodyFontFamily: string,
    _bodyFontStyle: string,
    _bodyAlign: string,
    bodyFontSize: number,
    bodySpacing: number,

    // Title
    // lines of text that form the title
    title: string[],
    titleFontColor: Color,
    _titleFontFamily: string,
    _titleFontStyle: string,
    titleFontSize: number,
    _titleAlign: string,
    titleSpacing: number,
    titleMarginBottom: number,

    // Footer
    // lines of text that form the footer
    footer: string[],
    footerFontColor: Color,
    _footerFontFamily: string,
    _footerFontStyle: string,
    footerFontSize: number,
    _footerAlign: string,
    footerSpacing: number,
    footerMarginTop: number,

    // Appearance
    caretSize: number,
    caretPadding: number,
    cornerRadius: number,
    backgroundColor: Color,

    // colors to render for each item in body[]. This is the color of the squares in the tooltip
    labelColors: Color[],
    labelTextColors: Color[],

    // 0 opacity is a hidden tooltip
    opacity: number,
    legendColorBackground: Color,
    displayColors: boolean,
    borderColor: Color,
    borderWidth: number
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

        if (!params.tooltipModel.opacity) {
            Tooltip.remove(tooltip);

            return;
        }

        if (params.tooltipModel.body) {
            Tooltip.render(tooltip, params.markUp);
        }

        const position = chart.getBoundingClientRect();

        const tooltipStyling = new Styling(params.tooltipModel, tooltip, params.tooltipTop, params.tooltipLeft, position);
        Tooltip.elemStyling(tooltipStyling);
    }

    public static createTooltip(id: string): HTMLElement {
        let tooltipEl = document.getElementById(id);

        if (!tooltipEl) {
            tooltipEl = document.createElement('div');
            tooltipEl.id = id;
            document.body.appendChild(tooltipEl);
        }

        return tooltipEl;
    }

    public static remove(tooltipEl: HTMLElement): void {
        document.body.removeChild(tooltipEl);
    }

    private static render(tooltip: HTMLElement, markUp: string): void {
        tooltip.innerHTML = markUp;
    }

    private static elemStyling(elemStyling: Styling): void {
        elemStyling.element.style.opacity = StylingConstants.tooltipOpacity;
        elemStyling.element.style.position = StylingConstants.tooltipPosition;
        elemStyling.element.style.left = `${elemStyling.chartPosition.left + elemStyling.tooltipModel.caretX - elemStyling.leftPosition}px`;
        elemStyling.element.style.top = `${elemStyling.chartPosition.top + window.pageYOffset + elemStyling.tooltipModel.caretY - elemStyling.topPosition}px`;
    }
}

/**
 * Stores data for chart's tooltip
 */
export class ChartTooltipData {
    public date: string;
    public value: string;

    public constructor(stamp: DataStamp) {
        const size = new Size(stamp.value, 1);

        this.date = stamp.intervalStart.toLocaleDateString('en-US', { day: '2-digit', month: 'short' });
        this.value = `${size.formattedBytes} ${size.label}`;
    }
}

/**
 * RenderChart contains definition for renderChart and addPlugin, that can be used to cast
 * a derived chart type, with `(this as unknown as RenderChart).renderChart`
 */
export interface RenderChart {
    renderChart<A, B>(A, B): void
    addPlugin (plugin?: Record<string, (chart) => void>): void
}
