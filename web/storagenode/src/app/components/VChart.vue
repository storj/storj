// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<script lang="ts">
import * as VueChart from 'vue-chartjs';
import { Component, Prop, Vue, Watch } from 'vue-property-decorator';

import { ChartData } from '@/app/types/chartData';

class DayShowingConditions {
    public readonly day: string;
    public readonly daysArray: string[];
    public readonly middleDateValue: number;
    public readonly isDateValueFirstOrLast: boolean;
    public readonly isAfterEighthDayOfTheMonth: boolean;

    public constructor(day: string, daysArray: string[]) {
        this.day = day;
        this.daysArray = daysArray;
        this.middleDateValue = this.countMiddleDateValue(daysArray);
        this.isDateValueFirstOrLast = this.isDayFirstOrLast(day, daysArray);
        this.isAfterEighthDayOfTheMonth = this.isDayAfterEighthDayOfTheMonth(daysArray);
    }

    private countMiddleDateValue(daysArray: string[]): number {
        return daysArray.length / 2;
    }

    private isDayFirstOrLast(day: string, daysArray: string[]): boolean {
        return day === daysArray[0] || day === daysArray[daysArray.length - 1];
    }

    private isDayAfterEighthDayOfTheMonth(daysArray: string[]): boolean {
        return daysArray.length > 8 && daysArray.length <= 31;
    }
}

@Component({
    extends: VueChart.Line
})
export default class VChart extends Vue {
    @Prop({default: '$'})
    private readonly currency: string;
    @Prop({default: () => { console.error('Tooltip constructor is undefined'); }, })
    private tooltipConstructor: (tooltipModel) => void;
    @Prop({default: {}})
    private readonly chartData: ChartData;

    @Watch('chartData')
    private onDataChange(news: object, old: object) {
        /**
         * renderChart method is inherited from BaseChart which is extended by VChart.Line
         */
        (this as any).renderChart(this.chartData, this.chartOptions);
    }

    public mounted(): void {
        /**
         * renderChart method is inherited from BaseChart which is extended by VChart.Line
         */
        (this as any).renderChart(this.chartData, this.chartOptions);
    }

    public get chartOptions(): object {
        const filterCallback = this.filterDaysDisplayed;
        return {
            responsive: false,
            maintainAspectRatios: false,

            legend: {
                display: false,
            },

            elements: {
                point: {
                    radius: 0,
                    hitRadius: 5,
                    hoverRadius: 5,
                    hoverBackgroundColor: '#4D72B7',
                }
            },

            scales: {
                yAxes: [{
                    display: true,
                    ticks: {
                        beginAtZero: true,
                        color: '#586C86',
                    },
                    gridLines: {
                        borderDash: [2, 5],
                        drawBorder: false
                    }
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

            layout: {
                padding: {
                    left: 25,
                }
            },

            tooltips: {
                enabled: false,

                custom: ((tooltipModel) => {
                    this.tooltipConstructor(tooltipModel);
                }),

                labels: {
                    enabled: true,
                }
            }
        };
    }

    private filterDaysDisplayed(day: string, dayIndex: string, labelArray: string[]): string | undefined {
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
            dayShowingConditions.daysArray[dayShowingConditions.middleDateValue - 1];

        return dayShowingConditions.isDateValueFirstOrLast || (isDaysAmountEven
            && dayShowingConditions.isAfterEighthDayOfTheMonth && isDateValueInMiddleInEvenAmount);
    }

    private areDaysShownOnNotEvenDaysAmount(dayShowingConditions: DayShowingConditions): boolean {
        const isDaysAmountNotEven = dayShowingConditions.daysArray.length % 2 !== 0;
        const isDateValueInMiddleInNotEvenAmount = dayShowingConditions.day
            === dayShowingConditions.daysArray[Math.floor(dayShowingConditions.middleDateValue)];

        return dayShowingConditions.isDateValueFirstOrLast || (isDaysAmountNotEven
            && dayShowingConditions.isAfterEighthDayOfTheMonth && isDateValueInMiddleInNotEvenAmount);
    }
}
</script>

<style scoped lang="scss"></style>
