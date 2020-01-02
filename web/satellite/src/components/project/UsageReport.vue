// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="usage-report-container">
        <div class="usage-report-container__header">
            <h1 class="usage-report-container__header__title">Report</h1>
        </div>
        <div class="usage-report-container__options-area">
            <div class="usage-report-container__options-area__option active" @click.prevent="onCurrentRollupClick">
                <p class="usage-report-container__options-area__option__label">Current Billing Period</p>
            </div>
            <div class="usage-report-container__options-area__option" @click.prevent="onPreviousRollupClick">
                <p class="usage-report-container__options-area__option__label">Previous Billing Period</p>
            </div>
            <div class="usage-report-container__options-area__option" @click.prevent.self="onCustomDateClick">
                <p class="usage-report-container__options-area__option__label" @click.prevent.self="onCustomDateClick">Custom Date Range</p>
                <VDatepicker
                    ref="datePicker"
                    :date="startTime"
                    @change="getDates"
                />
                <DatePickerIcon
                    class="usage-report-container__options-area__option__image"
                    @click.prevent="onCustomDateClick"
                />
            </div>
        </div>
        <div class="usage-report-container__main-area">
            <div class="usage-report-container__main-area__info-area">
                <div class="usage-report-container__main-area__info-area__item">
                    <h1 class="usage-report-container__main-area__info-area__item__title">Storage, {{storageDataDimension}}*h</h1>
                    <h2 class="usage-report-container__main-area__info-area__item__amount">{{storage}}</h2>
                </div>
                <div class="usage-report-container__main-area__info-area__item">
                    <h1 class="usage-report-container__main-area__info-area__item__title">Egress, {{egressDataDimension}}</h1>
                    <h2 class="usage-report-container__main-area__info-area__item__amount">{{egress}}</h2>
                </div>
                <div class="usage-report-container__main-area__info-area__item">
                    <h1 class="usage-report-container__main-area__info-area__item__title">Object Hours</h1>
                    <h2 class="usage-report-container__main-area__info-area__item__amount">{{objectsCount}}</h2>
                </div>
            </div>
            <div class="usage-report-container__main-area__footer">
                <p class="usage-report-container__main-area__footer__rollup-info">
                    Roll Up Period
                    <b class="usage-report-container__main-area__footer__rollup-info__bold-text">{{startDate}}</b> to
                    <b class="usage-report-container__main-area__footer__rollup-info__bold-text">{{endDate}}</b> (UTC +0)
                </p>
                <div class="usage-report-container__main-area__footer__report-area">
                    <p class="usage-report-container__main-area__footer__report-area__download-text">Download Advanced Report</p>
                    <DownloadReportIcon
                        class="usage-report-container__main-area__footer__report-area__image"
                        @click.prevent="onReportClick"
                    />
                </div>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VDatepicker from '@/components/common/VDatePicker.vue';

import DatePickerIcon from '@/../static/images/project/datePicker.svg';
import DownloadReportIcon from '@/../static/images/project/downloadReport.svg';

import { RouteConfig } from '@/router';
import { PROJECT_USAGE_ACTIONS } from '@/store/modules/usage';
import { DateRange } from '@/types/usage';
import { SegmentEvent } from '@/utils/constants/analyticsEventNames';
import { DateFormat } from '@/utils/datepicker';
import { toUnixTimestamp } from '@/utils/time';

@Component({
    components: {
        VDatepicker,
        DatePickerIcon,
        DownloadReportIcon,
    },
})
export default class UsageReport extends Vue {
    public readonly startTime = {
        time: null,
    };

    private readonly dateRange;

    public constructor() {
        super();

        const currentDate = new Date();
        const previousDate = new Date();
        previousDate.setMonth(currentDate.getMonth() - 1);

        this.dateRange = {
            startDate: previousDate,
            endDate: currentDate,
        };
    }

    public get startDate(): string {
        return DateFormat.getUSDate(this.$store.state.usageModule.startDate, '/');
    }

    public get endDate(): string {
        return DateFormat.getUSDate(this.$store.state.usageModule.endDate, '/');
    }

    public get storage(): string {
        return this.$store.state.usageModule.projectUsage.storage.formattedBytes;
    }

    public get egress(): string {
        return this.$store.state.usageModule.projectUsage.egress.formattedBytes;
    }

    public get objectsCount(): string {
        return this.$store.state.usageModule.projectUsage.objectCount.toPrecision(5);
    }

    public get storageDataDimension(): string {
        return this.$store.state.usageModule.projectUsage.storage.label;
    }

    public get egressDataDimension(): string {
        return this.$store.state.usageModule.projectUsage.egress.label;
    }

    public async mounted(): Promise<void> {
        try {
            this.$segment.track(SegmentEvent.REPORT_VIEWED, {
                project_id: this.$store.getters.selectedProject.id,
                start_date: this.dateRange.startDate,
                end_date: this.dateRange.endDate,
            });
            await this.$store.dispatch(PROJECT_USAGE_ACTIONS.FETCH_CURRENT_ROLLUP);
        } catch (error) {
            await this.$notify.error(`Unable to fetch project usage. ${error.message}`);
        }
    }

    public async beforeRouteLeave(to, from, next): Promise<void> {
        try {
            await this.$store.dispatch(PROJECT_USAGE_ACTIONS.FETCH_CURRENT_ROLLUP, this.dateRange);
        } catch (error) {
            await this.$notify.error(error.message);
        }

        const buttons = [...(document as any).querySelectorAll('.usage-report-container__options-area__option')];
        buttons.forEach(option => {
            option.classList.remove('active');
        });

        buttons[0].classList.add('active');
        next();
    }

    public onBackClick(): void {
        this.$router.push(RouteConfig.ProjectOverview.path);
    }

    public async onCurrentRollupClick(event: any): Promise<void> {
        this.onButtonClickAction(event);

        try {
            this.$segment.track(SegmentEvent.REPORT_VIEWED, {
                project_id: this.$store.getters.selectedProject.id,
                start_date: this.dateRange.startDate,
                end_date: this.dateRange.endDate,
            });
            await this.$store.dispatch(PROJECT_USAGE_ACTIONS.FETCH_CURRENT_ROLLUP);
        } catch (error) {
            await this.$notify.error(`Unable to fetch project usage. ${error.message}`);
        }
    }

    public async onPreviousRollupClick(event: any): Promise<void> {
        this.onButtonClickAction(event);

        try {
            this.$segment.track(SegmentEvent.REPORT_VIEWED, {
                project_id: this.$store.getters.selectedProject.id,
                start_date: this.dateRange.startDate,
                end_date: this.dateRange.endDate,
            });
            await this.$store.dispatch(PROJECT_USAGE_ACTIONS.FETCH_PREVIOUS_ROLLUP);
        } catch (error) {
            await this.$notify.error(`Unable to fetch project usage. ${error.message}`);
        }
    }

    public onCustomDateClick(event: any): void {
        (this as any).$refs.datePicker.showCheck();
        this.onButtonClickAction(event);
        this.$segment.track(SegmentEvent.REPORT_VIEWED, {
            project_id: this.$store.getters.selectedProject.id,
            start_date: this.dateRange.startDate,
            end_date: this.dateRange.endDate,
        });
    }

    public onReportClick(): void {
        const projectID = this.$store.getters.selectedProject.id;
        const startDate = this.$store.state.usageModule.startDate;
        const endDate = this.$store.state.usageModule.endDate;

        const url = new URL(location.origin);

        url.pathname = 'usage-report';
        url.searchParams.append('projectID', projectID);
        url.searchParams.append('since', toUnixTimestamp(startDate).toString());
        url.searchParams.append('before', toUnixTimestamp(endDate).toString());

        this.$segment.track(SegmentEvent.REPORT_DOWNLOADED, {
            start_date: startDate,
            end_date: endDate,
            project_id: projectID,
        });

        window.open(url.href, '_blank');
    }

    public async getDates(datesArray: Date[]): Promise<void> {
        const firstDate = new Date(datesArray[0]);
        const secondDate = new Date(datesArray[1]);
        const isInverted = firstDate > secondDate;

        const startDate = isInverted ? secondDate : firstDate;
        const endDate = isInverted ? firstDate : secondDate;

        const dateRange: DateRange = new DateRange(startDate, endDate);

        try {
            await this.$store.dispatch(PROJECT_USAGE_ACTIONS.FETCH, dateRange);
        } catch (error) {
            await this.$notify.error(`Unable to fetch project usage. ${error.message}`);
        }
    }

    public toLocaleDateString(date: Date): string {
        return date.toLocaleDateString('en-US', {timeZone: 'UTC'});
    }

    private onButtonClickAction(event: any): void {
        let eventTarget = event.target;

        if (eventTarget.children.length === 0) {
            eventTarget = eventTarget.parentNode;
        }

        if (eventTarget.classList.contains('active')) {
            return;
        }

        this.changeActiveClass(eventTarget);
    }

    private changeActiveClass(target: any): void {
        this.removeActiveClass();
        target.classList.add('active');
    }

    private removeActiveClass(): void {
        const buttons = [...(document as any).querySelectorAll('.usage-report-container__options-area__option')];
        buttons.forEach(option => {
            option.classList.remove('active');
        });
    }
}
</script>

<style scoped lang="scss">
    .usage-report-container {
        position: relative;

        &__header {
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: flex-start;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 32px;
                line-height: 39px;
                color: #263549;
                margin: 0;
                user-select: none;
            }
        }

        &__options-area {
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: flex-end;
            height: 100%;
            margin-top: 39px;

            &__option {
                display: flex;
                flex-direction: row;
                align-items: center;
                justify-content: center;
                width: 271px;
                height: 100%;
                background-color: #fff;
                border: solid 1px #f2f2f2;
                border-radius: 6px;
                cursor: pointer;
                margin-left: 20px;

                &__label {
                    font-family: 'font_medium', sans-serif;
                    font-size: 16px;
                    line-height: 23px;
                    color: #354049;
                    user-select: none;
                }

                &__image {
                    margin-left: 10px;
                }

                &.active {
                    background-color: #2683ff;

                    .usage-report-container__options-area__option__label {
                        color: #fff;
                    }

                    .usage-report-container__options-area__option__image {

                        .usage-report-svg-path {
                            fill: #fff !important;
                        }
                    }
                }
            }
        }

        &__main-area {
            display: flex;
            flex-direction: column;
            margin-top: 33px;

            &__info-area {
                display: flex;
                flex-direction: row;
                align-items: center;
                justify-content: space-between;
                height: 40vh;

                &__item {
                    max-width: 454px;
                    max-height: 393px;
                    width: 100%;
                    height: 100%;
                    background-color: #fff;
                    display: flex;
                    flex-direction: column;
                    align-items: center;
                    justify-content: center;
                    border-radius: 6px;
                    margin-right: 20px;

                    &:last-child {
                        margin-right: 0;
                    }

                    &__title {
                        font-family: 'font_regular', sans-serif;
                        font-size: 18px;
                        line-height: 24px;
                        color: #354049;
                        margin-block-start: 0;
                        margin-block-end: 0;
                        user-select: none;
                    }

                    &__amount {
                        margin-top: 22px;
                        font-family: 'font_medium', sans-serif;
                        font-size: 44px;
                        line-height: 67px;
                        color: #354049;
                        margin-block-start: 0.5em;
                        margin-block-end: 0;
                    }
                }
            }

            &__footer {
                padding: 0 32px;
                height: 86px;
                margin-top: 24px;
                background-color: #fff;
                border-radius: 6px;
                display: flex;
                flex-direction: row;
                align-items: center;
                justify-content: space-between;

                &__rollup-info {
                    font-family: 'font_regular', sans-serif;
                    font-size: 16px;
                    line-height: 21px;
                    color: #afb7c1;

                    &__bold-text {
                        font-family: 'font_medium', sans-serif;
                        color: #354049;
                    }
                }

                &__report-area {
                    display: flex;
                    flex-direction: row;
                    align-items: center;
                    justify-content: space-between;

                    &__download-text {
                        font-family: 'font_medium', sans-serif;
                        font-weight: bold;
                        font-size: 16px;
                        line-height: 21px;
                        color: #354049;
                        margin-right: 30px;
                        user-select: none;
                    }

                    &__image {
                        cursor: pointer;

                        .background {
                            fill: #e2ecf7;
                        }

                        .blue {
                            fill: #2683ff;
                        }
                    }

                    &__image:hover {

                        .background {
                            fill: #2683ff;
                        }

                        .blue {
                            fill: #fff;
                        }
                    }
                }
            }
        }
    }

    @media screen and (max-width: 1600px) {

        .usage-report-container {

            &__header {

                &__title {
                    font-size: 24px;
                    line-height: 29px;
                }
            }

            &__options-area {

                &__option {
                    width: 236px;

                    &__label {
                        font-size: 16px;
                        line-height: 23px;
                    }
                }
            }

            &__main-area {

                &__info-area {

                    &__item {

                        &__title {
                            font-size: 14px;
                            line-height: 19px;
                        }

                        &__amount {
                            font-size: 36px;
                            line-height: 54px;
                        }
                    }
                }

                &__footer {

                    &__rollup-info {
                        font-size: 16px;
                        line-height: 21px;
                    }

                    &__report-area {

                        &__download-text {
                            font-size: 16px;
                            line-height: 23px;
                        }
                    }
                }
            }
        }
    }

    @media screen and (max-width: 1350px) {

        .usage-report-container {

            &__options-area {

                &__option {
                    width: 200px;

                    &__label {
                        font-size: 14px;
                        line-height: 19px;
                    }

                    &:last-child {
                        width: 60px;
                        height: 47px;

                        .usage-report-container__options-area__option__label {
                            display: none;
                        }

                        .usage-report-container__options-area__option__image {
                            margin: 0;
                        }
                    }
                }
            }
        }
    }
</style>
