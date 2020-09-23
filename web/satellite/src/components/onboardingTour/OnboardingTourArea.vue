// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="tour-area">
        <div class="tour-area__info-bar" v-show="isInfoBarVisible && isPaywallEnabled">
            <div class="tour-area__info-bar__message">
                <b class="tour-area__info-bar__message__bold">Try Tardigrade with 50 GB Free after adding a payment method.</b>
                <p class="tour-area__info-bar__message__regular"> Cancel before your credit runs out and you’ll never be billed.</p>
            </div>
            <CloseImage class="tour-area__info-bar__close-img" @click="disableInfoBar"/>
        </div>
        <div class="tour-area__content">
            <ProgressBar
                :is-paywall-enabled="isPaywallEnabled"
                :is-add-payment-step="isAddPaymentState"
                :is-create-project-step="isCreateProjectState"
                :is-create-api-key-step="isCreatApiKeyState"
                :is-upload-data-step="isUploadDataState"
            />
            <OverviewStep
                v-if="isDefaultState && isPaywallEnabled"
                @setAddPaymentState="setAddPaymentState"
            />
            <OverviewStepNoPaywall
                v-if="isDefaultState && !isPaywallEnabled"
                @setCreateProjectState="setCreateProjectState"
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
            <img
                v-if="isAddPaymentState"
                class="tour-area__content__tardigrade"
                src="@/../static/images/onboardingTour/tardigrade.png"
                alt="tardigrade image"
            >
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import ProgressBar from '@/components/onboardingTour/ProgressBar.vue';
import AddPaymentStep from '@/components/onboardingTour/steps/AddPaymentStep.vue';
import CreateApiKeyStep from '@/components/onboardingTour/steps/CreateApiKeyStep.vue';
import CreateProjectStep from '@/components/onboardingTour/steps/CreateProjectStep.vue';
import OverviewStep from '@/components/onboardingTour/steps/OverviewStep.vue';
import OverviewStepNoPaywall from '@/components/onboardingTour/steps/OverviewStepNoPaywall.vue';
import UploadDataStep from '@/components/onboardingTour/steps/UploadDataStep.vue';

import CheckedImage from '@/../static/images/common/checked.svg';
import CloseImage from '@/../static/images/onboardingTour/close.svg';

import { RouteConfig } from '@/router';
import { TourState } from '@/utils/constants/onboardingTourEnums';

@Component({
    components: {
        OverviewStepNoPaywall,
        UploadDataStep,
        CreateApiKeyStep,
        CreateProjectStep,
        AddPaymentStep,
        ProgressBar,
        OverviewStep,
        CheckedImage,
        CloseImage,
    },
})

export default class OnboardingTourArea extends Vue {
    public areaState: number = TourState.DEFAULT;
    public isInfoBarVisible: boolean = true;

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
            this.disableInfoBar();
            this.setCreateApiKeyState();

            return;
        }

        if (this.$store.state.paymentsModule.creditCards.length > 0) {
            this.disableInfoBar();
            this.setCreateProjectState();

            return;
        }

        if (this.$store.getters.isTransactionProcessing || this.$store.getters.isBalancePositive) {
            this.setAddPaymentState();
        }
    }

    /**
     * Indicates if paywall is enabled.
     */
    public get isPaywallEnabled(): boolean {
        return this.$store.state.paymentsModule.isPaywallEnabled;
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
        this.disableInfoBar();
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
     * Disables info bar visibility.
     */
    public disableInfoBar(): void {
        this.isInfoBarVisible = false;
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
        width: 100%;

        &__info-bar {
            display: flex;
            align-items: center;
            justify-content: space-between;
            width: calc(100% - 60px);
            padding: 10px 30px;
            background-color: #7c8794;

            &__message {
                display: flex;
                align-items: center;

                &__bold,
                &__regular {
                    margin: 0 10px 0 0;
                    font-family: 'font_regular', sans-serif;
                    font-size: 14px;
                    line-height: 21px;
                    color: #fff;
                    word-break: break-word;
                }
            }

            &__close-img {
                cursor: pointer;
                min-width: 18px;
            }
        }

        &__content {
            padding: 0 100px 80px 100px;
            position: relative;

            &__tardigrade {
                position: absolute;
                left: 50%;
                bottom: 0;
                transform: translate(-50%);
            }
        }
    }

    @media screen and (max-width: 1550px) {

        .tour-area {

            &__content {
                padding: 0 50px 80px 50px;
            }
        }
    }

    @media screen and (max-width: 1000px) {

        .tour-area {

            &__content {
                padding: 0 25px 80px 25px;
            }
        }
    }
</style>
