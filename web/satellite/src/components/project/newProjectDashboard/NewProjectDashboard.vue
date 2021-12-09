// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div ref="dashboard" class="project-dashboard">
        <h1 class="project-dashboard__title">Dashboard</h1>
        <VLoader v-if="isDataFetching" class="project-dashboard__loader" width="100px" height="100px" />
        <p v-if="!isDataFetching && limits.objectCount" class="project-dashboard__subtitle">
            Your
            <span class="project-dashboard__subtitle__value">{{ limits.objectCount }} objects</span>
            are stored in
            <span class="project-dashboard__subtitle__value">{{ limits.segmentCount }} segments</span>
            around the world
        </p>
        <template v-if="!isDataFetching && !limits.objectCount">
            <p class="project-dashboard__subtitle">
                Welcome to Storj :) <br> You’re ready to experience the future of cloud storage
            </p>
            <VButton
                class="project-dashboard__upload-button"
                label="Upload"
                width="100px"
                height="40px"
                :on-press="onUploadClick"
            />
        </template>
        <div class="project-dashboard__stats-header">
            <h2 class="project-dashboard__stats-header__title">Project Stats</h2>
            <div class="project-dashboard__stats-header__buttons">
                <DateRangeSelection
                    :since="chartsSinceDate"
                    :before="chartsBeforeDate"
                    :on-date-pick="onChartsDateRangePick"
                    :is-open="isChartsDatePicker"
                    :toggle="toggleChartsDatePicker"
                />
                <VButton
                    v-if="!isProAccount"
                    label="Upgrade Plan"
                    width="114px"
                    height="40px"
                    font-size="13px"
                    :on-press="onUpgradeClick"
                />
                <VButton
                    v-else
                    class="new-project-button"
                    label="New Project"
                    width="130px"
                    height="40px"
                    font-size="13px"
                    :on-press="onCreateProjectClick"
                    :is-white="true"
                >
                    <template #icon>
                        <NewProjectIcon />
                    </template>
                </VButton>
            </div>
        </div>
        <div class="project-dashboard__charts">
            <div class="project-dashboard__charts__container">
                <h3 class="project-dashboard__charts__container__title">Storage</h3>
                <VLoader v-if="isDataFetching" class="project-dashboard__charts__container__loader" height="40px" width="40px" />
                <template v-else>
                    <p class="project-dashboard__charts__container__info">
                        Using {{ usedLimitFormatted(limits.storageUsed) }} of {{ usedLimitFormatted(limits.storageLimit) }}
                    </p>
                    <DashboardChart
                        name="storage"
                        :width="chartWidth"
                        :height="170"
                        :data="storageUsage"
                        :since="chartsSinceDate"
                        :before="chartsBeforeDate"
                        background-color="#E6EDF7"
                        border-color="#D7E8FF"
                        point-border-color="#003DC1"
                    />
                </template>
            </div>
            <div class="project-dashboard__charts__container">
                <h3 class="project-dashboard__charts__container__title">Bandwidth</h3>
                <VLoader v-if="isDataFetching" class="project-dashboard__charts__container__loader" height="40px" width="40px" />
                <template v-else>
                    <p class="project-dashboard__charts__container__info">
                        Using {{ usedLimitFormatted(limits.bandwidthUsed) }} of {{ usedLimitFormatted(limits.bandwidthLimit) }}
                    </p>
                    <DashboardChart
                        name="bandwidth"
                        :width="chartWidth"
                        :height="170"
                        :data="bandwidthUsage"
                        :since="chartsSinceDate"
                        :before="chartsBeforeDate"
                        background-color="#FFE0E7"
                        border-color="#FFC0CF"
                        point-border-color="#FF458B"
                    />
                </template>
            </div>
        </div>
        <div class="project-dashboard__info">
            <InfoContainer
                title="Billing"
                :subtitle="status"
                :value="estimatedCharges | centsToDollars"
                :is-data-fetching="isDataFetching"
            >
                <template #side-value>
                    <p class="project-dashboard__info__label">Will be charged during next billing period</p>
                </template>
            </InfoContainer>
            <InfoContainer
                class="project-dashboard__info__middle"
                title="Objects"
                :subtitle="`Updated ${now}`"
                :value="limits.objectCount"
                :is-data-fetching="isDataFetching"
            >
                <template #side-value>
                    <p class="project-dashboard__info__label">Total of {{ usedLimitFormatted(limits.storageUsed) }}</p>
                </template>
            </InfoContainer>
            <InfoContainer
                title="Segments"
                :subtitle="`Updated ${now}`"
                :value="limits.segmentCount"
                :is-data-fetching="isDataFetching"
            >
                <template #side-value>
                    <a
                        class="project-dashboard__info__link"
                        href="https://docs.storj.io/dcs/billing-payment-and-accounts-1/pricing/billing-and-payment"
                        target="_blank"
                        rel="noopener noreferrer"
                    >
                        Learn more ->
                    </a>
                </template>
            </InfoContainer>
        </div>
        <div class="project-dashboard__stats-header">
            <p class="project-dashboard__stats-header__title">Buckets</p>
        </div>
        <VLoader v-if="areBucketsFetching" />
        <BucketArea v-else />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { PROJECTS_ACTIONS } from "@/store/modules/projects";
import { PAYMENTS_ACTIONS, PAYMENTS_MUTATIONS } from "@/store/modules/payments";
import { APP_STATE_ACTIONS } from "@/utils/constants/actionNames";
import { BUCKET_ACTIONS } from "@/store/modules/buckets";
import { RouteConfig } from "@/router";
import { DataStamp, ProjectLimits } from "@/types/projects";
import { Dimensions, Size } from "@/utils/bytesSize";
import { ChartUtils } from "@/utils/chart";

import VLoader from "@/components/common/VLoader.vue";
import InfoContainer from "@/components/project/newProjectDashboard/InfoContainer.vue";
import DashboardChart from "@/components/project/newProjectDashboard/DashboardChart.vue";
import VButton from "@/components/common/VButton.vue";
import DateRangeSelection from "@/components/project/newProjectDashboard/DateRangeSelection.vue";
import BucketArea from '@/components/project/buckets/BucketArea.vue';

import NewProjectIcon from "@/../static/images/project/newProject.svg";

// @vue/component
@Component({
    components: {
        VLoader,
        VButton,
        InfoContainer,
        DashboardChart,
        DateRangeSelection,
        NewProjectIcon,
        BucketArea,
    }
})
export default class NewProjectDashboard extends Vue {
    public now = new Date().toLocaleDateString('en-US');
    public isDataFetching = true;
    public areBucketsFetching = true;

    public chartWidth = 0;

    public $refs: {
        dashboard: HTMLDivElement;
    }

    /**
     * Lifecycle hook after initial render.
     * Fetches project limits.
     */
    public async mounted(): Promise<void> {
        if (!this.$store.getters.selectedProject.id) {
            await this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.OverviewStep).path);

            return;
        }

        window.addEventListener('resize', this.recalculateChartWidth)
        this.recalculateChartWidth();

        try {
            const now = new Date()
            const past = new Date()
            past.setDate(past.getDate() - 30)

            await this.$store.dispatch(PROJECTS_ACTIONS.FETCH_DAILY_DATA, {since: past, before: now});
            await this.$store.dispatch(PROJECTS_ACTIONS.GET_LIMITS, this.$store.getters.selectedProject.id);
            await this.$store.dispatch(PAYMENTS_ACTIONS.GET_PROJECT_USAGE_AND_CHARGES_CURRENT_ROLLUP);

            this.isDataFetching = false;
        } catch (error) {
            await this.$notify.error(error.message);
        }

        const FIRST_PAGE = 1;

        try {
            await this.$store.dispatch(BUCKET_ACTIONS.FETCH, FIRST_PAGE);

            this.areBucketsFetching = false;
        } catch (error) {
            await this.$notify.error(error.message);
        }
    }

    /**
     * Lifecycle hook before component destruction.
     * Removes event on window resizing.
     */
    public beforeDestroy(): void {
        window.removeEventListener('resize', this.recalculateChartWidth);
    }

    /**
     * Used container size recalculation for charts resizing.
     */
    public recalculateChartWidth(): void {
        // sixty pixels.
        const additionalPaddingRight = 60;
        this.chartWidth = this.$refs.dashboard.getBoundingClientRect().width / 2 - additionalPaddingRight;
    }

    /**
     * Holds on upgrade button click logic.
     */
    public onUpgradeClick(): void {
        this.$store.commit(PAYMENTS_MUTATIONS.TOGGLE_IS_ADD_PM_MODAL_SHOWN);
    }

    /**
     * Holds on create project button click logic.
     */
    public onCreateProjectClick(): void {
        this.$router.push(RouteConfig.CreateProject.path);
    }

    /**
     * Holds on upload button click logic.
     */
    public onUploadClick(): void {
        this.$router.push(RouteConfig.Buckets.path).catch(() => {return;})
    }

    /**
     * Returns formatted amount.
     */
    public usedLimitFormatted(value: number): string {
        return this.formattedValue(new Size(value, 2));
    }

    /**
     * toggleChartsDatePicker holds logic for toggling charts date picker.
     */
    public toggleChartsDatePicker(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_CHARTS_DATEPICKER_DROPDOWN);
    }

    /**
     * onChartsDateRangePick holds logic for choosing date range for charts.
     * Fetches new data for specific date range.
     * @param dateRange
     */
    public async onChartsDateRangePick(dateRange: Date[]): Promise<void> {
        try {
            await this.$store.dispatch(PROJECTS_ACTIONS.FETCH_DAILY_DATA, {since: dateRange[0], before: dateRange[1]})
        } catch (error) {
            await this.$notify.error(error.message);
        }
    }

    /**
     * Indicates if charts date picker is shown.
     */
    public get isChartsDatePicker(): boolean {
        return this.$store.state.appStateModule.appState.isChartsDatePickerShown;
    }

    /**
     * Returns current limits from store.
     */
    public get limits(): ProjectLimits {
        return this.$store.state.projectsModule.currentLimits;
    }

    /**
     * Returns status string based on account status.
     */
    public get status(): string {
        return this.isProAccount ? 'Pro Account' : 'Free Account';
    }

    /**
     * Returns pro account status from store.
     */
    public get isProAccount(): boolean {
        return this.$store.getters.user.paidTier;
    }

    /**
     * Returns user's projects count from store.
     */
    public get ownProjectsCount(): number {
        return this.$store.getters.projectsCount;
    }

    /**
     * estimatedCharges returns estimated charges summary for selected project.
     */
    public get estimatedCharges(): number {
        return this.$store.state.paymentsModule.priceSummaryForSelectedProject;
    }

    /**
     * Returns storage chart data from store.
     */
    public get storageUsage(): DataStamp[] {
        return ChartUtils.populateEmptyUsage(this.$store.state.projectsModule.storageChartData, this.chartsSinceDate, this.chartsBeforeDate);
    }

    /**
     * Returns bandwidth chart data from store.
     */
    public get bandwidthUsage(): DataStamp[] {
        return ChartUtils.populateEmptyUsage(this.$store.state.projectsModule.bandwidthChartData, this.chartsSinceDate, this.chartsBeforeDate);
    }

    /**
     * Returns charts since date from store.
     */
    public get chartsSinceDate(): Date {
        return this.$store.state.projectsModule.chartDataSince;
    }

    /**
     * Returns charts before date from store.
     */
    public get chartsBeforeDate(): Date {
        return this.$store.state.projectsModule.chartDataBefore;
    }

    /**
     * Formats value to needed form and returns it.
     */
    private formattedValue(value: Size): string {
        switch (value.label) {
        case Dimensions.Bytes:
            return '0';
        default:
            return `${value.formattedBytes.replace(/\\.0+$/, '')}${value.label}`;
        }
    }
}
</script>

<style scoped lang="scss">
    .project-dashboard {
        padding: 56px 55px 56px 40px;
        height: calc(100% - 112px);
        max-width: calc(100vw - 280px - 95px);
        background-image: url('../../../../static/images/project/background.png');
        background-position: top right;
        background-size: 70%;
        background-repeat: no-repeat;
        font-family: 'font_regular', sans-serif;

        &__loader {
            display: inline-block;
        }

        &__title {
            font-family: 'font_medium', sans-serif;
            font-size: 16px;
            line-height: 24px;
            color: #000;
            margin-bottom: 64px;
        }

        &__subtitle {
            font-family: 'font_bold', sans-serif;
            font-size: 28px;
            line-height: 36px;
            letter-spacing: -0.02em;
            color: #000;
            max-width: 365px;

            &__value {
                text-decoration: underline;
                text-underline-position: under;
                text-decoration-color: #00e366;
            }
        }

        &__upload-button {
            margin-top: 24px;
        }

        &__stats-header {
            display: flex;
            align-items: center;
            justify-content: space-between;
            margin: 65px 0 16px 0;

            &__title {
                font-family: 'font_Bold', sans-serif;
                font-size: 24px;
                line-height: 31px;
                letter-spacing: -0.02em;
                color: #000;
            }

            &__buttons {
                display: flex;
                align-items: center;

                > .container {
                    margin-left: 16px;
                }
            }
        }

        &__charts {
            display: flex;
            align-items: center;

            &__container {
                width: 100%;
                background-color: #fff;
                box-shadow: 0 0 32px rgba(0, 0, 0, 0.04);
                border-radius: 10px;

                &__title {
                    margin: 16px 0 2px 24px;
                    font-family: 'font_medium', sans-serif;
                    font-size: 18px;
                    line-height: 27px;
                    color: #000;
                }

                &__loader {
                    margin-bottom: 15px;
                }

                &__info {
                    margin: 2px 0 0 24px;
                    font-weight: 600;
                    font-size: 14px;
                    line-height: 20px;
                    color: #000;
                }
            }

            > *:first-child {
                margin-right: 20px;
            }
        }

        &__info {
            display: flex;
            align-items: center;
            margin-top: 16px;

            &__middle {
                margin: 0 16px;
            }

            &__label,
            &__link {
                font-weight: 500;
                font-size: 14px;
                line-height: 20px;
                color: #000;
            }

            &__link {
                text-decoration: underline !important;
                text-underline-position: under;

                &:visited {
                    color: #000;
                }
            }
        }
    }

    .new-project-button {

        svg {
            margin-right: 9px;
        }

        &:hover {

            svg path {
                fill: #fff;
            }
        }
    }

    @media screen and (max-width: 1280px) {

        .project-dashboard {
            max-width: calc(100vw - 86px - 95px);
        }
    }
</style>
