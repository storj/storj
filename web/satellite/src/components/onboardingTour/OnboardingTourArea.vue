// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="tour-area">
        <ProgressBar
            :is-create-project-step="isCreateProjectState"
            :is-create-api-key-step="isCreatApiKeyState"
            :is-upload-data-step="isUploadDataState"
        />
        <OverviewStep
            v-if="isDefaultState"
            @setAddPaymentState="setAddPaymentState"
        />
        <AddPaymentStep
            v-if="isAddPaymentState"
            @setProjectState="setCreateProjectState"
        />
        <CreateProjectStep
            v-if="isCreateProjectState"
            @setApiKeyState="setCreateApiKeyState"
        />
        <CreateApiKeyStep
            v-if="isCreatApiKeyState"
            @setUploadDataState="setUploadDataState"
        />
        <UploadDataStep v-if="isUploadDataState"/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import ProgressBar from '@/components/onboardingTour/ProgressBar.vue';
import AddPaymentStep from '@/components/onboardingTour/steps/AddPaymentStep.vue';
import CreateApiKeyStep from '@/components/onboardingTour/steps/CreateApiKeyStep.vue';
import CreateProjectStep from '@/components/onboardingTour/steps/CreateProjectStep.vue';
import OverviewStep from '@/components/onboardingTour/steps/OverviewStep.vue';
import UploadDataStep from '@/components/onboardingTour/steps/UploadDataStep.vue';

import CheckedImage from '@/../static/images/common/checked.svg';

import { RouteConfig } from '@/router';
import { TourState } from '@/utils/constants/onboardingTourEnums';

@Component({
    components: {
        UploadDataStep,
        CreateApiKeyStep,
        CreateProjectStep,
        AddPaymentStep,
        ProgressBar,
        OverviewStep,
        CheckedImage,
    },
})

export default class OnboardingTourArea extends Vue {
    public areaState: number = TourState.DEFAULT;

    /**
     * Lifecycle hook after initial render.
     * Sets area to needed state.
     */
    public mounted(): void {
        if (this.userHasProject && this.userHasApiKeys) {
            try {
                this.$router.push(RouteConfig.ProjectDashboard.path);
            } catch (error) {
                return;
            }

            return;
        }

        if (this.userHasProject && !this.userHasApiKeys) {
            this.setCreateApiKeyState();

            return;
        }

        if (this.$store.state.paymentsModule.creditCards.length > 0) {
            this.setCreateProjectState();

            return;
        }

        if (this.$store.getters.isTransactionProcessing || this.$store.getters.isTransactionCompleted) {
            this.setAddPaymentState();
        }
    }

    /**
     * Indicates if area is in default state.
     */
    public get isDefaultState(): boolean {
        return this.areaState === TourState.DEFAULT;
    }

    /**
     * Indicates if area is in adding payment method state.
     */
    public get isAddPaymentState(): boolean {
        return this.areaState === TourState.ADDING_PAYMENT;
    }

    /**
     * Indicates if area is in creating project state.
     */
    public get isCreateProjectState(): boolean {
        return this.areaState === TourState.PROJECT;
    }

    /**
     * Indicates if area is in api key state.
     */
    public get isCreatApiKeyState(): boolean {
        return this.areaState === TourState.API_KEY;
    }

    /**
     * Indicates if area is in upload data state.
     */
    public get isUploadDataState(): boolean {
        return this.areaState === TourState.UPLOAD;
    }

    /**
     * Sets area's state to adding payment method state.
     */
    public setAddPaymentState(): void {
        this.areaState = TourState.ADDING_PAYMENT;
    }

    /**
     * Sets area's state to creating project state.
     */
    public setCreateProjectState(): void {
        this.areaState = TourState.PROJECT;
    }

    /**
     * Sets area's state to creating api key state.
     */
    public setCreateApiKeyState(): void {
        this.areaState = TourState.API_KEY;
    }

    /**
     * Sets area's state to upload data state.
     */
    public setUploadDataState(): void {
        this.areaState = TourState.UPLOAD;
    }

    /**
     * Indicates if user has at least one project.
     */
    private get userHasProject(): boolean {
        return this.$store.state.projectsModule.projects.length > 0;
    }

    /**
     * Indicates if user has at least one API key.
     */
    private get userHasApiKeys(): boolean {
        return this.$store.state.apiKeysModule.page.apiKeys.length > 0;
    }
}
</script>

<style scoped lang="scss">
    .tour-area {
        padding: 0 100px;
    }

    @media screen and (max-width: 1380px) {

        .tour-area {
            padding: 0 50px;
        }
    }

    @media screen and (max-width: 1000px) {

        .tour-area {
            padding: 0 25px;
        }
    }
</style>
