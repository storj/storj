// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <CLIFlowContainer
        :on-back-click="onBackClick"
        :on-next-click="onNextClick"
        :is-loading="isLoading || areBucketNamesFetching"
        title="Access Permissions"
    >
        <template #icon>
            <Icon />
        </template>
        <template #content class="permissions">
            <div class="permissions__select">
                <p class="permissions__select__label">Select buckets to grant permission:</p>
                <VLoader v-if="areBucketNamesFetching" width="50px" height="50px" />
                <BucketsSelection v-else />
            </div>
            <div class="permissions__bucket-bullets">
                <div
                    v-for="(name, index) in selectedBucketNames"
                    :key="index"
                    class="permissions__bucket-bullets__container"
                >
                    <BucketNameBullet :name="name" />
                </div>
            </div>
            <div class="permissions__select">
                <p class="permissions__select__label">Choose permissions to allow:</p>
                <PermissionsSelect />
            </div>
            <div class="permissions__select">
                <p class="permissions__select__label">Duration of this access grant:</p>
                <DurationSelection />
            </div>
        </template>
    </CLIFlowContainer>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/router';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { AnalyticsHttpApi } from '@/api/analytics';

import CLIFlowContainer from '@/components/onboardingTour/steps/common/CLIFlowContainer.vue';
import PermissionsSelect from '@/components/onboardingTour/steps/cliFlow/PermissionsSelect.vue';
import BucketNameBullet from '@/components/accessGrants/permissions/BucketNameBullet.vue';
import BucketsSelection from '@/components/accessGrants/permissions/BucketsSelection.vue';
import VLoader from '@/components/common/VLoader.vue';
import DurationSelection from '@/components/accessGrants/permissions/DurationSelection.vue';

import Icon from '@/../static/images/onboardingTour/accessGrant.svg';

// @vue/component
@Component({
    components: {
        CLIFlowContainer,
        PermissionsSelect,
        BucketNameBullet,
        BucketsSelection,
        VLoader,
        DurationSelection,
        Icon,
    },
})
export default class AGPermissions extends Vue {
    private worker: Worker;
    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    public areBucketNamesFetching = true;
    public isLoading = true;

    /**
     * Lifecycle hook after initial render.
     * Checks if clean api key was generated during previous step.
     * Fetches all existing bucket names.
     * Initializes web worker's onmessage functionality.
     */
    public async mounted(): Promise<void> {
        if (!this.cleanAPIKey) {
            this.isLoading = false;
            await this.onBackClick();

            return;
        }

        this.setWorker();

        try {
            await this.$store.dispatch(BUCKET_ACTIONS.FETCH_ALL_BUCKET_NAMES);

            this.areBucketNamesFetching = false;
        } catch (error) {
            await this.$notify.error(`Unable to fetch all bucket names. ${error.message}`, AnalyticsErrorEventSource.ONBOARDING_PERMISSIONS_STEP);
        }

        this.isLoading = false;
    }

    /**
     * Sets local worker with worker instantiated in store.
     * Also sets worker's onmessage and onerror logic.
     */
    public setWorker(): void {
        this.worker = this.$store.state.accessGrantsModule.accessGrantsWebWorker;
        this.worker.onerror = (error: ErrorEvent) => {
            this.$notify.error(error.message, AnalyticsErrorEventSource.ONBOARDING_PERMISSIONS_STEP);
        };
    }

    /**
     * Holds on back button click logic.
     * Navigates to previous screen.
     */
    public async onBackClick(): Promise<void> {
        if (this.isLoading) return;

        this.analytics.pageVisit(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.AGName)).path);
        await this.$router.push({ name: RouteConfig.AGName.name });
    }

    /**
     * Holds on next button click logic.
     */
    public async onNextClick(): Promise<void> {
        if (this.isLoading) return;

        this.isLoading = true;

        try {
            const restrictedKey = await this.generateRestrictedKey();
            this.$store.commit(APP_STATE_MUTATIONS.SET_ONB_API_KEY, restrictedKey);

            await this.$notify.success('Restrictions were set successfully.');
        } catch (error) {
            await this.$notify.error(error.message, AnalyticsErrorEventSource.ONBOARDING_PERMISSIONS_STEP);
            return;
        } finally {
            this.isLoading = false;
        }

        await this.$store.commit(APP_STATE_MUTATIONS.SET_ONB_API_KEY_STEP_BACK_ROUTE, this.$route.path);
        this.analytics.pageVisit(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.APIKey)).path);
        await this.$router.push({ name: RouteConfig.APIKey.name });
    }

    /**
     * Generates and returns restricted key from clean API key.
     */
    private async generateRestrictedKey(): Promise<string> {
        let permissionsMsg = {
            'type': 'SetPermission',
            'isDownload': this.storedIsDownload,
            'isUpload': this.storedIsUpload,
            'isList': this.storedIsList,
            'isDelete': this.storedIsDelete,
            'buckets': this.selectedBucketNames,
            'apiKey': this.cleanAPIKey,
        };

        if (this.notBeforePermission) permissionsMsg = Object.assign(permissionsMsg, { 'notBefore': this.notBeforePermission.toISOString() });
        if (this.notAfterPermission) permissionsMsg = Object.assign(permissionsMsg, { 'notAfter': this.notAfterPermission.toISOString() });

        await this.worker.postMessage(permissionsMsg);

        const grantEvent: MessageEvent = await new Promise(resolve => this.worker.onmessage = resolve);
        if (grantEvent.data.error) {
            throw new Error(grantEvent.data.error);
        }

        this.analytics.eventTriggered(AnalyticsEvent.API_KEY_GENERATED);

        return grantEvent.data.value;
    }

    /**
     * Returns selected bucket names.
     */
    public get selectedBucketNames(): string[] {
        return this.$store.state.accessGrantsModule.selectedBucketNames;
    }

    /**
     * Returns clean API key from store.
     */
    private get cleanAPIKey(): string {
        return this.$store.state.appStateModule.viewsState.onbCleanApiKey;
    }

    /**
     * Returns download permission from store.
     */
    private get storedIsDownload(): boolean {
        return this.$store.state.accessGrantsModule.isDownload;
    }

    /**
     * Returns upload permission from store.
     */
    private get storedIsUpload(): boolean {
        return this.$store.state.accessGrantsModule.isUpload;
    }

    /**
     * Returns list permission from store.
     */
    private get storedIsList(): boolean {
        return this.$store.state.accessGrantsModule.isList;
    }

    /**
     * Returns delete permission from store.
     */
    private get storedIsDelete(): boolean {
        return this.$store.state.accessGrantsModule.isDelete;
    }

    /**
     * Returns not before date permission from store.
     */
    private get notBeforePermission(): Date | null {
        return this.$store.state.accessGrantsModule.permissionNotBefore;
    }

    /**
     * Returns not after date permission from store.
     */
    private get notAfterPermission(): Date | null {
        return this.$store.state.accessGrantsModule.permissionNotAfter;
    }
}
</script>

<style scoped lang="scss">
    .permissions {

        &__select {
            width: 287px;
            padding: 0 98.5px;

            &__label {
                font-family: 'font_medium', sans-serif;
                margin: 20px 0 8px;
                font-size: 14px;
                line-height: 20px;
                color: var(--c-grey-6);
            }
        }

        &__bucket-bullets {
            display: flex;
            align-items: flex-start;
            width: calc(100% - 197px);
            padding: 0 98.5px;
            flex-wrap: wrap;

            &__container {
                display: flex;
                margin-top: 5px;
            }
        }
    }

    :deep(.buckets-selection),
    :deep(.duration-selection) {
        width: 287px;
        margin-left: 0;
    }
</style>
