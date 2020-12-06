// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="add-storj-state">
        <div class="add-storj-state__container">
            <p class="add-storj-state__container__bonus-info">
                Deposit STORJ Token to your account and receive a 10% bonus, or $10 for every $100.
            </p>
            <div class="add-storj-state__container__deposit-area">
                <p class="add-storj-state__container__deposit-area__info">
                    <b>Please Note:</b> Your first deposit of $10 or more in STORJ Token is applied to your account
                    after Coin Payments verifies payment<br/><br/>50GB are your starting project limits. Increased
                    amounts are available
                    <a
                        class="add-storj-state__container__deposit-area__info__request-link"
                        href="https://support.tardigrade.io/hc/en-us/requests/new?ticket_form_id=360000683212"
                        target="_blank"
                        rel="noopener noreferrer"
                    >
                        per request.
                    </a>
                </p>
                <PayingStep
                    v-if="isDefaultState"
                    @toggleIsLoading="toggleIsLoading"
                    @setVerifyingState="setVerifyingState"
                />
                <VerifyingStep
                    v-if="isVerifyingState"
                    @setDefaultState="setDefaultState"
                />
                <VerifiedStep v-if="isVerifiedState"/>
            </div>
            <div class="add-storj-state__container__blur" v-if="isLoading"/>
        </div>
        <p class="add-storj-state__next-label">Next</p>
        <VButton
            width="252px"
            height="48px"
            label="Create an Acess Grant"
            :on-press="createAccessGrant"
            :is-disabled="isButtonDisabled"
        />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VButton from '@/components/common/VButton.vue';
import PayingStep from '@/components/onboardingTour/steps/paymentStates/tokenSubSteps/PayingStep.vue';
import VerifiedStep from '@/components/onboardingTour/steps/paymentStates/tokenSubSteps/VerifiedStep.vue';
import VerifyingStep from '@/components/onboardingTour/steps/paymentStates/tokenSubSteps/VerifyingStep.vue';

import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { ProjectFields } from '@/types/projects';
import { PM_ACTIONS } from '@/utils/constants/actionNames';
import { SegmentEvent } from '@/utils/constants/analyticsEventNames';
import { AddingStorjState } from '@/utils/constants/onboardingTourEnums';

@Component({
    components: {
        VerifiedStep,
        VerifyingStep,
        PayingStep,
        VButton,
    },
})

export default class AddStorjState extends Vue {
    private readonly TOGGLE_IS_LOADING: string = 'toggleIsLoading';
    private readonly SET_CREATE_GRANT_STEP: string = 'setCreateGrantStep';

    public isLoading: boolean = false;
    public addingTokenState: number = AddingStorjState.DEFAULT;

    /**
     * Lifecycle hook after initial render.
     * Sets area to needed state.
     */
    public beforeMount(): void {
        switch (true) {
            case this.$store.getters.isBalancePositive:
                this.setVerifiedState();

                return;
            case this.$store.getters.isTransactionProcessing:
                this.setVerifyingState();

                return;
            default:
                this.setDefaultState();
        }
    }

    /**
     * Create untitled project and starts creating access grant process.
     */
    public async createAccessGrant(): Promise<void> {
        if (this.isLoading) return;

        this.toggleIsLoading();

        try {
            const FIRST_PAGE = 1;
            const UNTITLED_PROJECT_NAME = 'Untitled Project';
            const UNTITLED_PROJECT_DESCRIPTION = '___';
            const project = new ProjectFields(
                UNTITLED_PROJECT_NAME,
                UNTITLED_PROJECT_DESCRIPTION,
                this.$store.getters.user.id,
            );
            const createdProject = await this.$store.dispatch(PROJECTS_ACTIONS.CREATE, project);
            const createdProjectId = createdProject.id;

            this.$segment.track(SegmentEvent.PROJECT_CREATED, {
                project_id: createdProjectId,
            });

            await this.$store.dispatch(PROJECTS_ACTIONS.SELECT, createdProjectId);
            await this.$store.dispatch(PM_ACTIONS.CLEAR);
            await this.$store.dispatch(PM_ACTIONS.FETCH, FIRST_PAGE);
            await this.$store.dispatch(PAYMENTS_ACTIONS.GET_PAYMENTS_HISTORY);
            await this.$store.dispatch(PAYMENTS_ACTIONS.GET_BALANCE);
            await this.$store.dispatch(PAYMENTS_ACTIONS.GET_PROJECT_USAGE_AND_CHARGES_CURRENT_ROLLUP);
            await this.$store.dispatch(PROJECTS_ACTIONS.GET_LIMITS, createdProjectId);
            await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.CLEAR);
            await this.$store.dispatch(BUCKET_ACTIONS.CLEAR);

            this.toggleIsLoading();

            this.$emit(this.SET_CREATE_GRANT_STEP);
        } catch (error) {
            await this.$notify.error(error.message);
            this.toggleIsLoading();
        }
    }

    /**
     * Sets area to default state.
     */
    public setDefaultState(): void {
        this.addingTokenState = AddingStorjState.DEFAULT;
    }

    /**
     * Sets area to verifying state.
     */
    public setVerifyingState(): void {
        this.addingTokenState = AddingStorjState.VERIFYING;
    }

    /**
     * Sets area to verified state.
     */
    public setVerifiedState(): void {
        this.addingTokenState = AddingStorjState.VERIFIED;
    }

    /**
     * Toggles area's loading state.
     */
    public toggleIsLoading(): void {
        this.isLoading = !this.isLoading;
        this.$emit(this.TOGGLE_IS_LOADING);
    }

    /**
     * Indicates if area is in default state.
     */
    public get isDefaultState(): boolean {
        return this.addingTokenState === AddingStorjState.DEFAULT;
    }

    /**
     * Indicates if area is in verifying state.
     */
    public get isVerifyingState(): boolean {
        return this.addingTokenState === AddingStorjState.VERIFYING;
    }

    /**
     * Indicates if area is in verified state.
     */
    public get isVerifiedState(): boolean {
        return this.addingTokenState === AddingStorjState.VERIFIED;
    }

    /**
     * Indicates if button is disabled.
     */
    public get isButtonDisabled(): boolean {
        return !this.$store.getters.canUserCreateFirstProject;
    }
}
</script>

<style scoped lang="scss">
    p,
    h2 {
        margin: 0;
    }

    .add-storj-state {
        font-family: 'font_regular', sans-serif;
        width: 100%;
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: space-between;
        margin-bottom: 20px;

        &__container {
            width: calc(100% - 90px);
            padding: 20px 45px 45px 45px;
            background-color: #fff;
            border-radius: 0 0 8px 8px;
            position: relative;

            &__bonus-info {
                padding: 20px 30px;
                width: calc(100% - 60px);
                background-color: #edf4fe;
                border-radius: 6px;
                font-size: 14px;
                line-height: 20px;
                color: #7b8eab;
            }

            &__deposit-area {
                width: 100%;
                display: flex;
                align-items: flex-start;
                justify-content: space-between;
                margin-top: 30px;

                &__info {
                    background-color: rgba(245, 246, 250, 0.65);
                    padding: 35px;
                    border-radius: 6px;
                    font-size: 12px;
                    line-height: 17px;
                    color: #7b8eab;
                    width: calc(40% - 70px);
                    margin-right: 40px;

                    &__request-link {
                        text-decoration: underline;
                    }
                }
            }

            &__blur {
                position: absolute;
                top: 0;
                left: 0;
                height: 100%;
                width: 100%;
                background-color: rgba(229, 229, 229, 0.2);
                z-index: 100;
            }
        }

        &__next-label {
            font-weight: normal;
            font-size: 16px;
            line-height: 26px;
            color: #768394;
            margin: 35px 0;
        }
    }
</style>