// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<script lang="ts">
import * as VueChart from 'vue-chartjs';
import { Component, Prop, Vue, Watch } from 'vue-property-decorator';

import { ChartData, RenderChart } from '@/app/types/chart';

class DayShowingConditions {
    public readonly day: string;
    public readonly daysArray: string[];

    public constructor(day: string, daysArray: string[]) {
        this.day = day;
        this.daysArray = daysArray;
    }

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
    extends: VueChart.Line,
})
export default class VChart extends Vue {
    @Prop({ default: '$' })
    private readonly currency: string;
    @Prop({ default: () => () => { console.error('Tooltip constructor is undefined'); } })
    private tooltipConstructor: (tooltipModel) => void;
    @Prop({ default: {} })
    private readonly chartData: ChartData;

    @Watch('chartData')
    private onDataChange(_news: Record<string, unknown>, _old: Record<string, unknown>) {
        /**
         * renderChart method is inherited from BaseChart which is extended by VChart.Line
         */
        (this as unknown as RenderChart).renderChart(this.chartData, this.chartOptions);
    }

    public mounted(): void {
        /**
         * renderChart method is inherited from BaseChart which is extended by VChart.Line
         */
        (this as unknown as RenderChart).renderChart(this.chartData, this.chartOptions);
    }

    public get chartOptions(): Record<string, unknown> {
        const filterCallback = this.filterDaysDisplayed;
        const ticksColor = this.$vuetify.theme.dark? '#242d40' : '#e1e3e6';

        return {
            responsive: false,
            maintainAspectRatios: false,
            legend: {
                display: false,
            },
            elements: {
                point: {
                    radius: 0,
                    hoverRadius: 0,
                    hitRadius: 500,
                },
            },
            scales: {
                yAxes: [{
                    display: true,
                    ticks: {
                        beginAtZero: true,
                        color: ticksColor,
                    },
                    gridLines: {
                        borderDash: [2, 5],
                        drawBorder: false,
                        color: ticksColor,
                    },
                }],
                xAxes: [{
                    display: true,
                    ticks: {
                        fontFamily: 'font_regular',
                        autoSkip: false,
                        maxRotation: 0,
                        minRotation: 0,
                        callback: filterCallback,
                        color: ticksColor,
                    },
                    gridLines: {
                        display: false,
                    },
                }],
            },
            layout: {
                padding: {
                    left: 25,
                },
            },
            tooltips: {
                enabled: false,

                custom: (tooltipModel) => {
                    this.tooltipConstructor(tooltipModel);
                },

                labels: {
                    enabled: true,
                },
            },
        };
    }

    private filterDaysDisplayed(day: string, _dayIndex: string, labelArray: string[]): string | undefined {
        const eighthDayOfTheMonth = 8;
        const isBeforeEighthDayOfTheMonth = labelArray.length <= eighthDayOfTheMonth;
        const dayShowingConditions = new DayShowingConditions(day, labelArray);

        if (isBeforeEighthDayOfTheMonth || this.areDaysShownOnEvenDaysAmount(dayShowingConditions)
            || this.areDaysShownOnNotEvenDaysAmount(dayShowingConditions)) {
            return day;
        }
    }

    private areDaysShownOnEvenDaysAmount(dayShowingConditions: DayShowingConditions): boolean {
        const isDaysAmountEven = dayShowingConditions.daysArray.length % 2 === 0;
        const isDateValueInMiddleInEvenAmount = dayShowingConditions.day ===
            dayShowingConditions.daysArray[dayShowingConditions.countMiddleDateValue() - 1];

        return dayShowingConditions.isDayFirstOrLast() || isDaysAmountEven
            && dayShowingConditions.isDayAfterEighthDayOfTheMonth() && isDateValueInMiddleInEvenAmount;
    }

    private areDaysShownOnNotEvenDaysAmount(dayShowingConditions: DayShowingConditions): boolean {
        const isDaysAmountNotEven = dayShowingConditions.daysArray.length % 2 !== 0;
        const isDateValueInMiddleInNotEvenAmount = dayShowingConditions.day
            === dayShowingConditions.daysArray[Math.floor(dayShowingConditions.countMiddleDateValue())];

        return dayShowingConditions.isDayFirstOrLast() || isDaysAmountNotEven
            && dayShowingConditions.isDayAfterEighthDayOfTheMonth() && isDateValueInMiddleInNotEvenAmount;
    }
}
</script>
