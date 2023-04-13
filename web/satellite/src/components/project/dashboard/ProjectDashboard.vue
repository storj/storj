// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="project-dashboard">
        <div class="project-dashboard__heading">
            <h1 class="project-dashboard__heading__title" aria-roledescription="title">{{ selectedProject.name }}</h1>
            <project-ownership-tag :project="selectedProject" />
        </div>

        <p class="project-dashboard__message">
            Expect a delay of a few hours between network activity and the latest dashboard stats.
        </p>
        <DashboardFunctionalHeader :loading="isDataFetching || areBucketsFetching" />
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
            <div ref="chartContainer" class="project-dashboard__charts__container">
                <div class="project-dashboard__charts__container__header">
                    <h3 class="project-dashboard__charts__container__header__title">Storage</h3>
                </div>
                <VLoader v-if="isDataFetching" class="project-dashboard__charts__container__loader" height="40px" width="40px" />
                <template v-else>
                    <StorageChart
                        :width="chartWidth"
                        :height="170"
                        :data="storageUsage"
                        :since="chartsSinceDate"
                        :before="chartsBeforeDate"
                    />
                </template>
            </div>
            <div class="project-dashboard__charts__container">
                <div class="project-dashboard__charts__container__header">
                    <h3 class="project-dashboard__charts__container__header__title">Bandwidth</h3>
                    <div class="project-dashboard__charts__container__header__right">
                        <span class="project-dashboard__charts__container__header__right__allocated-color" />
                        <p class="project-dashboard__charts__container__header__right__allocated-label">Allocated</p>
                        <span class="project-dashboard__charts__container__header__right__settled-color" />
                        <p class="project-dashboard__charts__container__header__right__settled-label">Settled</p>
                        <VInfo class="project-dashboard__charts__container__header__right__info">
                            <template #icon>
                                <InfoIcon />
                            </template>
                            <template #message>
                                <p class="project-dashboard__charts__container__header__right__info__message">
                                    The bandwidth allocated takes few hours to be settled.
                                    <a
                                        class="project-dashboard__charts__container__header__right__info__message__link"
                                        href="https://docs.storj.io/dcs/billing-payment-and-accounts-1/pricing/billing-and-payment#bandwidth-fee"
                                        target="_blank"
                                        rel="noopener noreferrer"
                                    >
                                        Learn more
                                    </a>
                                </p>
                            </template>
                        </VInfo>
                    </div>
                </div>
                <VLoader v-if="isDataFetching" class="project-dashboard__charts__container__loader" height="40px" width="40px" />
                <template v-else>
                    <BandwidthChart
                        :width="chartWidth"
                        :height="170"
                        :settled-data="settledBandwidthUsage"
                        :allocated-data="allocatedBandwidthUsage"
                        :since="chartsSinceDate"
                        :before="chartsBeforeDate"
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
                title="Objects"
                :subtitle="`Updated ${now}`"
                :value="limits.objectCount.toString()"
                :is-data-fetching="isDataFetching"
            >
                <template #side-value>
                    <p class="project-dashboard__info__label" aria-roledescription="total-storage">
                        Total of {{ usedLimitFormatted(limits.storageUsed) }}
                    </p>
                </template>
            </InfoContainer>
            <InfoContainer
                title="Segments"
                :subtitle="`Updated ${now}`"
                :value="limits.segmentCount.toString()"
                :is-data-fetching="isDataFetching"
            >
                <template #side-value>
                    <a
                        class="project-dashboard__info__link"
                        href="https://docs.storj.io/dcs/billing-payment-and-accounts-1/pricing#segments"
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
        <BucketsTable :is-loading="areBucketsFetching" />
        <EncryptionBanner v-if="!isServerSideEncryptionBannerHidden" :hide="hideBanner" />
    </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';

import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { RouteConfig } from '@/router';
import { DataStamp, Project, ProjectLimits } from '@/types/projects';
import { Dimensions, Size } from '@/utils/bytesSize';
import { ChartUtils } from '@/utils/chart';
import { AnalyticsHttpApi } from '@/api/analytics';
import { LocalData } from '@/utils/localData';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { APP_STATE_DROPDOWNS, MODALS } from '@/utils/constants/appStatePopUps';
import { useNotify, useRouter, useStore } from '@/utils/hooks';
import { useUsersStore } from '@/store/modules/usersStore';
import { useBillingStore } from '@/store/modules/billingStore';
import { useAppStore } from '@/store/modules/appStore';

import VLoader from '@/components/common/VLoader.vue';
import InfoContainer from '@/components/project/dashboard/InfoContainer.vue';
import StorageChart from '@/components/project/dashboard/StorageChart.vue';
import BandwidthChart from '@/components/project/dashboard/BandwidthChart.vue';
import DashboardFunctionalHeader from '@/components/project/dashboard/DashboardFunctionalHeader.vue';
import VButton from '@/components/common/VButton.vue';
import DateRangeSelection from '@/components/project/dashboard/DateRangeSelection.vue';
import VInfo from '@/components/common/VInfo.vue';
import BucketsTable from '@/components/objects/BucketsTable.vue';
import EncryptionBanner from '@/components/objects/EncryptionBanner.vue';
import ProjectOwnershipTag from '@/components/project/ProjectOwnershipTag.vue';

import NewProjectIcon from '@/../static/images/project/newProject.svg';
import InfoIcon from '@/../static/images/project/infoIcon.svg';

const appStore = useAppStore();
const billingStore = useBillingStore();
const usersStore = useUsersStore();
const store = useStore();
const notify = useNotify();
const router = useRouter();

const now = new Date().toLocaleDateString('en-US');
const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

const isDataFetching = ref<boolean>(true);
const areBucketsFetching = ref<boolean>(true);
const isServerSideEncryptionBannerHidden = ref<boolean>(true);
const chartWidth = ref<number>(0);
const chartContainer = ref<HTMLDivElement>();

/**
 * Indicates if charts date picker is shown.
 */
const isChartsDatePicker = computed((): boolean => {
    return appStore.state.viewsState.activeDropdown === APP_STATE_DROPDOWNS.CHART_DATE_PICKER;
});

/**
 * Returns current limits from store.
 */
const limits = computed((): ProjectLimits => {
    return store.state.projectsModule.currentLimits;
});

/**
 * Returns status string based on account status.
 */
const status = computed((): string => {
    return isProAccount.value ? 'Pro Account' : 'Free Account';
});

/**
 * Returns pro account status from store.
 */
const isProAccount = computed((): boolean => {
    return usersStore.state.user.paidTier;
});

/**
 * estimatedCharges returns estimated charges summary for selected project.
 */
const estimatedCharges = computed((): number => {
    const projID: string = store.getters.selectedProject.id;
    const charges = billingStore.state.projectCharges;
    return charges.getProjectPrice(projID);
});

/**
 * Returns storage chart data from store.
 */
const storageUsage = computed((): DataStamp[] => {
    return ChartUtils.populateEmptyUsage(
        store.state.projectsModule.storageChartData, chartsSinceDate.value, chartsBeforeDate.value,
    );
});

/**
 * Returns settled bandwidth chart data from store.
 */
const settledBandwidthUsage = computed((): DataStamp[] => {
    return ChartUtils.populateEmptyUsage(
        store.state.projectsModule.settledBandwidthChartData, chartsSinceDate.value, chartsBeforeDate.value,
    );
});

/**
 * Returns allocated bandwidth chart data from store.
 */
const allocatedBandwidthUsage = computed((): DataStamp[] => {
    return ChartUtils.populateEmptyUsage(
        store.state.projectsModule.allocatedBandwidthChartData, chartsSinceDate.value, chartsBeforeDate.value,
    );
});

/**
 * Returns charts since date from store.
 */
const chartsSinceDate = computed((): Date => {
    return store.state.projectsModule.chartDataSince;
});

/**
 * Returns charts before date from store.
 */
const chartsBeforeDate = computed((): Date => {
    return store.state.projectsModule.chartDataBefore;
});

/**
 * Indicates if user has just logged in.
 */
const hasJustLoggedIn = computed((): boolean => {
    return appStore.state.viewsState.hasJustLoggedIn;
});

/**
 * Indicates if bucket was created.
 */
const bucketWasCreated = computed((): boolean => {
    const status = LocalData.getBucketWasCreatedStatus();
    if (status !== null) {
        return status;
    }

    return false;
});

/**
 * get selected project from store
 */
const selectedProject = computed((): Project => {
    return store.getters.selectedProject;
});

/**
 * Hides server-side encryption banner.
 */
function hideBanner(): void {
    isServerSideEncryptionBannerHidden.value = true;
    LocalData.setServerSideEncryptionBannerHidden(true);
}

/**
 * Used container size recalculation for charts resizing.
 */
function recalculateChartWidth(): void {
    chartWidth.value = chartContainer.value?.getBoundingClientRect().width || 0;
}

/**
 * Holds on upgrade button click logic.
 */
function onUpgradeClick(): void {
    appStore.updateActiveModal(MODALS.addPaymentMethod);
}

/**
 * Holds on create project button click logic.
 */
function onCreateProjectClick(): void {
    analytics.pageVisit(RouteConfig.CreateProject.path);
    router.push(RouteConfig.CreateProject.path);
}

/**
 * Returns formatted amount.
 */
function usedLimitFormatted(value: number): string {
    return formattedValue(new Size(value, 2));
}

/**
 * toggleChartsDatePicker holds logic for toggling charts date picker.
 */
function toggleChartsDatePicker(): void {
    appStore.toggleActiveDropdown(APP_STATE_DROPDOWNS.CHART_DATE_PICKER);
}

/**
 * onChartsDateRangePick holds logic for choosing date range for charts.
 * Fetches new data for specific date range.
 * @param dateRange
 */
async function onChartsDateRangePick(dateRange: Date[]): Promise<void> {
    const since = new Date(dateRange[0]);
    const before = new Date(dateRange[1]);
    before.setHours(23, 59, 59, 999);

    try {
        await store.dispatch(PROJECTS_ACTIONS.FETCH_DAILY_DATA, { since, before });
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.PROJECT_DASHBOARD_PAGE);
    }
}

/**
 * Formats value to needed form and returns it.
 */
function formattedValue(value: Size): string {
    switch (value.label) {
    case Dimensions.Bytes:
        return '0';
    default:
        return `${value.formattedBytes.replace(/\\.0+$/, '')}${value.label}`;
    }
}

/**
 * Lifecycle hook after initial render.
 * Fetches project limits.
 */
onMounted(async (): Promise<void> => {
    isServerSideEncryptionBannerHidden.value = LocalData.getServerSideEncryptionBannerHidden();

    if (!store.getters.selectedProject.id) {
        if (appStore.state.isAllProjectsDashboard) {
            await router.push(RouteConfig.AllProjectsDashboard.path);
            return;
        }
        const onboardingPath = RouteConfig.OnboardingTour.with(RouteConfig.FirstOnboardingStep).path;

        analytics.pageVisit(onboardingPath);
        await router.push(onboardingPath);

        return;
    }

    window.addEventListener('resize', recalculateChartWidth);
    recalculateChartWidth();

    try {
        const now = new Date();
        const past = new Date();
        past.setDate(past.getDate() - 30);

        await store.dispatch(PROJECTS_ACTIONS.GET_LIMITS, store.getters.selectedProject.id);
        if (hasJustLoggedIn.value) {
            if (limits.value.objectCount > 0) {
                appStore.updateActiveModal(MODALS.enterPassphrase);
                if (!bucketWasCreated.value) {
                    LocalData.setBucketWasCreatedStatus();
                }
            } else {
                appStore.updateActiveModal(MODALS.createProjectPassphrase);
            }

            appStore.toggleHasJustLoggedIn();
        }

        await store.dispatch(PROJECTS_ACTIONS.FETCH_DAILY_DATA, { since: past, before: now });
        await billingStore.getProjectUsageAndChargesCurrentRollup();
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.PROJECT_DASHBOARD_PAGE);
    } finally {
        isDataFetching.value = false;
    }

    const FIRST_PAGE = 1;

    try {
        await store.dispatch(BUCKET_ACTIONS.FETCH, FIRST_PAGE);

        areBucketsFetching.value = false;
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.PROJECT_DASHBOARD_PAGE);
    }
});

/**
 * Lifecycle hook before component destruction.
 * Removes event listener on window resizing.
 */
onBeforeUnmount((): void => {
    window.removeEventListener('resize', recalculateChartWidth);
});
</script>

<style scoped lang="scss">
    .project-dashboard {
        max-width: calc(100vw - 280px - 95px);
        background-image: url('../../../../static/images/project/background.png');
        background-position: top right;
        background-size: 70%;
        background-repeat: no-repeat;
        font-family: 'font_regular', sans-serif;

        &__heading {
            display: flex;
            gap: 10px;
            align-items: center;
            margin-bottom: 20px;

            &__title {
                font-family: 'font_medium', sans-serif;
                font-size: 16px;
                line-height: 24px;
                color: #000;
            }
        }

        &__message {
            font-size: 16px;
            line-height: 20px;
            color: #384b65;
            margin: 10px 0 64px;
        }

        &__stats-header {
            display: flex;
            align-items: center;
            justify-content: space-between;
            flex-wrap: wrap;
            margin: 63px -8px 14px;

            > * {
                margin: 2px 8px;
            }

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
            justify-content: space-between;

            &__container {
                width: calc((100% - 20px) / 2);
                background-color: #fff;
                box-shadow: 0 0 32px rgb(0 0 0 / 4%);
                border-radius: 10px;

                &__header {
                    display: flex;
                    align-items: flex-start;
                    justify-content: space-between;

                    &__title {
                        margin: 16px 0 2px 24px;
                        font-family: 'font_medium', sans-serif;
                        font-size: 18px;
                        line-height: 27px;
                        color: #000;
                    }

                    &__right {
                        display: flex;
                        align-items: center;
                        margin: 16px 16px 0 0;

                        &__allocated-color,
                        &__settled-color {
                            width: 10px;
                            height: 10px;
                            border-radius: 2px;
                        }

                        &__allocated-color {
                            background: var(--c-purple-2);
                        }

                        &__settled-color {
                            background: var(--c-purple-3);
                        }

                        &__allocated-label,
                        &__settled-label {
                            font-size: 14px;
                            line-height: 17px;
                            color: #000;
                            margin-left: 5px;
                        }

                        &__allocated-label {
                            margin-right: 16px;
                        }

                        &__settled-label {
                            margin-right: 11px;
                        }

                        &__info {
                            cursor: pointer;
                            max-height: 20px;

                            &__message {
                                font-size: 12px;
                                line-height: 18px;
                                text-align: center;
                                color: #fff;

                                &__link {
                                    text-decoration: underline !important;
                                    color: #fff;

                                    &:visited {
                                        color: #fff;
                                    }
                                }
                            }
                        }
                    }
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
        }

        &__info {
            display: flex;
            margin-top: 16px;
            justify-content: space-between;
            align-items: stretch;
            flex-wrap: wrap;

            .info-container {
                width: calc((100% - 32px) / 3);
                box-sizing: border-box;
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

        &__bucket-area {
            margin-top: 0;
        }
    }

    .new-project-button {

        &:hover svg :deep(path) {
            fill: #fff;
        }
    }

    :deep(.info__box) {
        width: 180px;
        left: calc(50% - 20px);
        top: calc(100% + 1px);
        cursor: default;
    }

    :deep(.info__box__message) {
        background: var(--c-grey-6);
        border-radius: 4px;
        padding: 8px;
        position: relative;
        right: 25%;
    }

    :deep(.info__box__arrow) {
        background: var(--c-grey-6);
        width: 10px;
        height: 10px;
        margin: 0 0 -2px 40px;
    }

    :deep(.range-selection__popup) {
        z-index: 1;
    }

    @media screen and (max-width: 1280px) {

        .project-dashboard {
            max-width: calc(100vw - 86px - 95px);
        }
    }

    @media screen and (max-width: 960px) {

        :deep(.range-selection__popup) {
            right: -148px;
        }
    }

    @media screen and (max-width: 768px) {

        .project-dashboard {

            &__stats-header {
                margin-bottom: 20px;
            }

            &__charts {
                flex-direction: column;

                &__container {
                    width: 100%;
                }

                &__container:first-child {
                    margin-right: 0;
                    margin-bottom: 22px;
                }
            }

            &__info {
                margin-top: 52px;

                > .info-container {
                    width: calc((100% - 25px) / 2);
                    margin-bottom: 24px;
                }

                > .info-container:last-child {
                    width: 100%;
                    margin-bottom: 0;
                }
            }
        }

        :deep(.range-selection__popup) {
            left: 0;
        }
    }

    @media screen and (max-width: 480px) {

        .project-dashboard {

            &__charts__container:first-child {
                margin-bottom: 20px;
            }

            &__info {
                margin-top: 32px;

                > .info-container {
                    width: 100%;
                    margin-bottom: 16px;
                }
            }
        }
    }
</style>
