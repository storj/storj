// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="project-dashboard">
        <div class="project-dashboard__heading">
            <h1 class="project-dashboard__heading__title" aria-roledescription="title">{{ selectedProject.name }}</h1>
            <project-ownership-tag :is-owner="selectedProject.ownerId === user.id" />
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
        <LimitsArea v-if="limitsAreaEnabled" :is-loading="isDataFetching" />
        <div class="project-dashboard__info">
            <InfoContainer
                :icon="BucketsIcon"
                title="Buckets"
                :subtitle="`Last update ${now}`"
                :value="bucketsCount.toString()"
                :is-data-fetching="areBucketsFetching"
            >
                <template #side-value>
                    <router-link :to="RouteConfig.Buckets.path" class="project-dashboard__info__link">
                        Go to buckets →
                    </router-link>
                </template>
            </InfoContainer>
            <InfoContainer
                :icon="GrantsIcon"
                title="Access Grants"
                :subtitle="`Last update ${now}`"
                :value="accessGrantsCount.toString()"
                :is-data-fetching="isDataFetching"
            >
                <template #side-value>
                    <router-link :to="RouteConfig.AccessGrants.path" class="project-dashboard__info__link">
                        Access management →
                    </router-link>
                </template>
            </InfoContainer>
            <InfoContainer
                :icon="TeamIcon"
                title="Users"
                :subtitle="`Last update ${now}`"
                :value="teamSize.toString()"
                :is-data-fetching="isDataFetching"
            >
                <template #side-value>
                    <p class="project-dashboard__info__link" @click="onInviteUsersClick">
                        Invite project users →
                    </p>
                </template>
            </InfoContainer>
            <InfoContainer
                :icon="BillingIcon"
                title="Billing"
                :subtitle="status"
                :value="isProAccount ? centsToDollars(estimatedCharges) : 'Free'"
                :is-data-fetching="isDataFetching"
            >
                <template #side-value>
                    <router-link
                        :to="RouteConfig.Account.with(RouteConfig.Billing.with(RouteConfig.BillingOverview)).path"
                        class="project-dashboard__info__link"
                    >
                        Go to billing →
                    </router-link>
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
import { useRouter } from 'vue-router';

import { RouteConfig } from '@/router';
import { DataStamp, Project, ProjectLimits } from '@/types/projects';
import { Dimensions, Size } from '@/utils/bytesSize';
import { ChartUtils } from '@/utils/chart';
import { AnalyticsHttpApi } from '@/api/analytics';
import { LocalData } from '@/utils/localData';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { APP_STATE_DROPDOWNS, MODALS } from '@/utils/constants/appStatePopUps';
import { useNotify } from '@/utils/hooks';
import { useUsersStore } from '@/store/modules/usersStore';
import { useBillingStore } from '@/store/modules/billingStore';
import { useAppStore } from '@/store/modules/appStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useProjectMembersStore } from '@/store/modules/projectMembersStore';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { centsToDollars } from '@/utils/strings';
import { User } from '@/types/users';

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
import LimitsArea from '@/components/project/dashboard/LimitsArea.vue';

import NewProjectIcon from '@/../static/images/project/newProject.svg';
import InfoIcon from '@/../static/images/project/infoIcon.svg';
import BucketsIcon from '@/../static/images/navigation/buckets.svg';
import GrantsIcon from '@/../static/images/navigation/accessGrants.svg';
import TeamIcon from '@/../static/images/navigation/users.svg';
import BillingIcon from '@/../static/images/navigation/billing.svg';

const configStore = useConfigStore();
const bucketsStore = useBucketsStore();
const appStore = useAppStore();
const billingStore = useBillingStore();
const usersStore = useUsersStore();
const projectsStore = useProjectsStore();
const pmStore = useProjectMembersStore();
const agStore = useAccessGrantsStore();
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
    return appStore.state.activeDropdown === APP_STATE_DROPDOWNS.CHART_DATE_PICKER;
});

/**
 * Returns current limits from store.
 */
const limits = computed((): ProjectLimits => {
    return projectsStore.state.currentLimits;
});

/**
 * Returns the whether the limits area is enabled.
 */
const limitsAreaEnabled = computed((): boolean => {
    return configStore.state.config.limitsAreaEnabled;
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
    const charges = billingStore.state.projectCharges;
    return charges.getProjectPrice(selectedProject.value.id);
});

/**
 * Returns storage chart data from store.
 */
const storageUsage = computed((): DataStamp[] => {
    return ChartUtils.populateEmptyUsage(
        projectsStore.state.storageChartData, chartsSinceDate.value, chartsBeforeDate.value,
    );
});

/**
 * Returns settled bandwidth chart data from store.
 */
const settledBandwidthUsage = computed((): DataStamp[] => {
    return ChartUtils.populateEmptyUsage(
        projectsStore.state.settledBandwidthChartData, chartsSinceDate.value, chartsBeforeDate.value,
    );
});

/**
 * Returns allocated bandwidth chart data from store.
 */
const allocatedBandwidthUsage = computed((): DataStamp[] => {
    return ChartUtils.populateEmptyUsage(
        projectsStore.state.allocatedBandwidthChartData, chartsSinceDate.value, chartsBeforeDate.value,
    );
});

/**
 * Returns charts since date from store.
 */
const chartsSinceDate = computed((): Date => {
    return projectsStore.state.chartDataSince;
});

/**
 * Returns charts before date from store.
 */
const chartsBeforeDate = computed((): Date => {
    return projectsStore.state.chartDataBefore;
});

/**
 * Indicates if user has just logged in.
 */
const hasJustLoggedIn = computed((): boolean => {
    return appStore.state.hasJustLoggedIn;
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
 * Returns user entity from store.
 */
const user = computed((): User => {
    return usersStore.state.user;
});

/**
 * Get selected project from store.
 */
const selectedProject = computed((): Project => {
    return projectsStore.state.selectedProject;
});

/**
 * Returns current team size from store.
 */
const teamSize = computed((): number => {
    return pmStore.state.page.totalCount;
});

/**
 * Returns access grants count from store.
 */
const accessGrantsCount = computed((): number => {
    return agStore.state.page.totalCount;
});

/**
 * Returns access grants count from store.
 */
const bucketsCount = computed((): number => {
    return bucketsStore.state.page.totalCount;
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
    appStore.updateActiveModal(MODALS.upgradeAccount);
}

/**
 * Holds on invite users CTA click logic.
 */
function onInviteUsersClick(): void {
    appStore.updateActiveModal(MODALS.addTeamMember);
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
        await projectsStore.getDailyProjectData({ since, before });
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

    const projectID = selectedProject.value.id;
    if (!projectID) {
        if (configStore.state.config.allProjectsDashboard) {
            await router.push(RouteConfig.AllProjectsDashboard.path);
            return;
        }
        const onboardingPath = RouteConfig.OnboardingTour.with(configStore.firstOnboardingStep).path;

        analytics.pageVisit(onboardingPath);
        router.push(onboardingPath);

        return;
    }

    window.addEventListener('resize', recalculateChartWidth);
    recalculateChartWidth();

    const FIRST_PAGE = 1;

    try {
        const now = new Date();
        const past = new Date();
        past.setDate(past.getDate() - 30);

        await projectsStore.getProjectLimits(projectID);
        if (hasJustLoggedIn.value) {
            if (limits.value.objectCount > 0) {
                if (usersStore.state.settings.passphrasePrompt) {
                    appStore.updateActiveModal(MODALS.enterPassphrase);
                }
                if (!bucketWasCreated.value) {
                    LocalData.setBucketWasCreatedStatus();
                }
            } else {
                if (usersStore.state.settings.passphrasePrompt) {
                    appStore.updateActiveModal(MODALS.createProjectPassphrase);
                }
            }

            appStore.toggleHasJustLoggedIn();
        }

        await Promise.all([
            projectsStore.getDailyProjectData({ since: past, before: now }),
            billingStore.getProjectUsageAndChargesCurrentRollup(),
            billingStore.getCoupon(),
            pmStore.getProjectMembers(FIRST_PAGE, projectID),
            agStore.getAccessGrants(FIRST_PAGE, projectID),
        ]);
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.PROJECT_DASHBOARD_PAGE);
    } finally {
        isDataFetching.value = false;
    }

    try {
        await bucketsStore.getBuckets(FIRST_PAGE, projectID);

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
        background-origin: content-box;
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
            margin: 44px -8px 14px;

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

                        @media screen and (width <= 390px) {
                            flex-direction: column;
                            align-items: flex-end;
                            row-gap: 5px;
                        }

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

                            @media screen and (width <= 390px) {
                                margin-right: 0;
                            }
                        }

                        &__settled-label {
                            margin-right: 11px;

                            @media screen and (width <= 390px) {
                                margin-right: 0;
                            }
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
                width: calc((100% - 32px) / 4);
                box-sizing: border-box;
            }

            @media screen and (width <= 1060px) {

                > .info-container {
                    width: calc((100% - 16px) / 2);
                    margin-bottom: 16px;
                }
            }

            @media screen and (width <= 600px) {

                > .info-container {
                    width: 100%;
                }
            }

            &__link {
                font-weight: 500;
                font-size: 14px;
                line-height: 20px;
                color: var(--c-black);
                cursor: pointer;
                text-decoration: underline !important;
                text-underline-position: under;

                &:visited {
                    color: var(--c-black);
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

    @media screen and (width <= 1280px) {

        .project-dashboard {
            max-width: calc(100vw - 86px - 95px);
        }
    }

    @media screen and (width <= 960px) {

        :deep(.range-selection__popup) {
            right: -148px;
        }
    }

    @media screen and (width <= 768px) {

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
        }

        :deep(.range-selection__popup) {
            left: 0;
        }
    }

    @media screen and (width <= 480px) {

        .project-dashboard {

            &__charts__container:first-child {
                margin-bottom: 20px;
            }
        }
    }
</style>
