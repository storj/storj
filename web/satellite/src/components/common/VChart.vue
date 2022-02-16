// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<script lang="ts">
import { Line } from 'vue-chartjs';
import { Component, Prop, Vue, Watch } from 'vue-property-decorator';

import { ChartData, RenderChart } from '@/types/chart';

/**
 * Used to filter days displayed on x-axis.
 */
class DayShowingConditions {
    public constructor(
        public day: string,
        public daysArray: string[],
    ) {}

    public countMiddleDateValue(): number {
        return this.daysArray.length / 2;
    }

    public isDayFirstOrLast(): boolean {
        return this.day === this.daysArray[0] || this.day === this.daysArray[this.daysArray.length - 1];
    }

    public isDayAfterEighthDayOfTheMonth(): boolean {
        return this.daysArray.length > 8 && this.daysArray.length <= 31;
    }
}

// @vue/component
@Component({
    extends: Line,
})
export default class VChart extends Vue {
    @Prop({ default: () => { console.error('Tooltip constructor is undefined'); } })
    private tooltipConstructor: (tooltipModel) => void;
    @Prop({ default: {} })
    private readonly chartData: ChartData;

    /**
     * Mounted hook after initial render.
     * Adds chart plugin to draw dashed line under data point.
     * Renders chart.
     */
    public mounted(): void {
        (this as unknown as RenderChart).addPlugin({
            afterDatasetsDraw: (chart): void => {
                if (chart.tooltip._active && chart.tooltip._active.length) {
                    const activePoint = chart.tooltip._active[0];
                    const ctx = chart.ctx;
                    const y_axis = chart.scales['y-axis-0'];
                    const tooltipPosition = activePoint.tooltipPosition();

                    ctx.save();
                    ctx.beginPath();
                    ctx.setLineDash([8, 5]);
                    ctx.moveTo(tooltipPosition.x, tooltipPosition.y + 12);
                    ctx.lineTo(tooltipPosition.x, y_axis.bottom);
                    ctx.lineWidth = 1;
                    ctx.strokeStyle = '#C8D3DE';
                    ctx.stroke();
                    ctx.restore();
                }
            }
        });
        (this as unknown as RenderChart).renderChart(this.chartData, this.chartOptions);
    }

    @Watch('chartData')
    private onDataChange(_news: Record<string, unknown>, _old: Record<string, unknown>) {
        /**
         * renderChart method is inherited from BaseChart which is extended by VChart.Line
         */
        (this as unknown as RenderChart).renderChart(this.chartData, this.chartOptions);
    }

    /**
     * Returns chart options.
     */
    public get chartOptions(): Record<string, unknown> {
        const filterCallback = this.filterDaysDisplayed;

        return {
            responsive: false,
            maintainAspectRatios: false,
            animation: false,
            hover: {
                animationDuration: 0
            },
            responsiveAnimationDuration: 0,
            legend: {
                display: false,
            },
            layout: {
                padding: {
                    top: 40,
                }
            },
            elements: {
                point: {
                    radius: this.chartData.labels.length === 1 ? 10 : 0,
                    hoverRadius: 10,
                    hitRadius: 8,
                },
            },
            scales: {
                yAxes: [{
                    display: false,
                }],
                xAxes: [{
                    display: true,
                    ticks: {
                        fontFamily: 'font_regular',
                        autoSkip: false,
                        maxRotation: 0,
                        minRotation: 0,
                        callback: filterCallback,
                    },
                    gridLines: {
                        display: false,
                    },
                }],
            },
            tooltips: {
                enabled: false,
                axis: 'x',
                custom: (tooltipModel) => {
                    this.tooltipConstructor(tooltipModel);
                },
            },
        };
    }

    /**
     * Used as callback to filter days displayed on chart.
     */
    private filterDaysDisplayed(day: string, _dayIndex: string, labelArray: string[]): string | undefined {
        const eighthDayOfTheMonth = 8;
        const isBeforeEighthDayOfTheMonth = labelArray.length <= eighthDayOfTheMonth;
        const dayShowingConditions = new DayShowingConditions(day, labelArray);

        if (isBeforeEighthDayOfTheMonth || this.areDaysShownOnEvenDaysAmount(dayShowingConditions)
            || this.areDaysShownOnNotEvenDaysAmount(dayShowingConditions)) {
            return day;
        }
    }

    /**
     * Indicates if days are shown on even days amount.
     */
    private areDaysShownOnEvenDaysAmount(dayShowingConditions: DayShowingConditions): boolean {
        const isDaysAmountEven = dayShowingConditions.daysArray.length % 2 === 0;
        const isDateValueInMiddleInEvenAmount = dayShowingConditions.day ===
            dayShowingConditions.daysArray[dayShowingConditions.countMiddleDateValue() - 1];

        return dayShowingConditions.isDayFirstOrLast() || (isDaysAmountEven
            && dayShowingConditions.isDayAfterEighthDayOfTheMonth() && isDateValueInMiddleInEvenAmount);
    }

    /**
     * Indicates if days are shown on not even days amount.
     */
    private areDaysShownOnNotEvenDaysAmount(dayShowingConditions: DayShowingConditions): boolean {
        const isDaysAmountNotEven = dayShowingConditions.daysArray.length % 2 !== 0;
        const isDateValueInMiddleInNotEvenAmount = dayShowingConditions.day
            === dayShowingConditions.daysArray[Math.floor(dayShowingConditions.countMiddleDateValue())];

        return dayShowingConditions.isDayFirstOrLast() || (isDaysAmountNotEven
            && dayShowingConditions.isDayAfterEighthDayOfTheMonth() && isDateValueInMiddleInNotEvenAmount);
    }
}
</script>
