// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { TooltipModel, ChartType } from 'chart.js';

import { DataStamp } from '@/types/projects';
import { Size } from '@/utils/bytesSize';

/**
 * TooltipParams holds tooltip's configuration
 */
export class TooltipParams {
    public constructor(
        public tooltipModel: TooltipModel<ChartType>,
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
        public tooltipModel: TooltipModel<ChartType>,
        public element: HTMLElement,
        public topPosition: number,
        public leftPosition: number,
        public chartPosition: DOMRect,
    ) {}
}

/**
 * TooltipId defines tooltip IDs.
 */
export enum TooltipId {
    Storage = 'storage-tooltip',
    Bandwidth = 'allocated-bandwidth-tooltip',
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
        elemStyling.element.style.top = `${elemStyling.chartPosition.top + window.scrollY + elemStyling.tooltipModel.caretY - elemStyling.topPosition}px`;
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
