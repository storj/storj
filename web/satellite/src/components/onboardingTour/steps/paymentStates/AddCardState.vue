// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="add-card-state">
        <div class="add-card-state__input-container">
            <StripeCardInput
                ref="stripeCardInput"
                :on-stripe-response-callback="addCard"
            />
            <div class="add-card-state__input-container__blur" v-if="isLoading"/>
        </div>
        <div class="add-card-state__security-info">
            <LockImage/>
            <span class="add-card-state__security-info__text">
                Your card is secured by 128-bit SSL and AES-256 encryption. Your information is secure.
            </span>
        </div>
        <p class="add-card-state__next-label">Next</p>
        <VButton
            label="Create an Access Grant"
            width="252px"
            height="48px"
            :is-disabled="isLoading"
            :on-press="onCreateGrantClick"
        />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import StripeCardInput from '@/components/account/billing/paymentMethods/StripeCardInput.vue';
import VButton from '@/components/common/VButton.vue';

import LockImage from '@/../static/images/account/billing/lock.svg';

import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { ProjectFields } from '@/types/projects';
import { PM_ACTIONS } from '@/utils/constants/actionNames';
import { SegmentEvent } from '@/utils/constants/analyticsEventNames';

const {
    ADD_CREDIT_CARD,
    GET_CREDIT_CARDS,
    GET_BALANCE,
} = PAYMENTS_ACTIONS;

interface StripeForm {
    onSubmit(): Promise<void>;
}

@Component({
    components: {
        StripeCardInput,
        LockImage,
        VButton,
    },
})

export default class AddCardState extends Vue {
    private readonly TOGGLE_IS_LOADING: string = 'toggleIsLoading';
    private readonly SET_CREATE_GRANT_STEP: string = 'setCreateGrantStep';

    public isLoading: boolean = false;
    public $refs!: {
        stripeCardInput: StripeCardInput & StripeForm;
    };

    /**
     * Provides card information to Stripe, creates untitled project and redirects to next step.
     */
    public async onCreateGrantClick(): Promise<void> {
        if (this.isLoading) return;

        this.isLoading = true;

        this.$emit(this.TOGGLE_IS_LOADING);

        try {
            await this.$refs.stripeCardInput.onSubmit();

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
            this.$segment.track(SegmentEvent.PAYMENT_METHOD_ADDED, {
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

            this.setDefaultState();

            this.$emit(this.SET_CREATE_GRANT_STEP);
        } catch (error) {
            await this.$notify.error(error.message);
            this.setDefaultState();
        }
    }

    /**
     * Adds card after Stripe confirmation.
     *
     * @param token from Stripe
     */
    public async addCard(token: string) {
        try {
            await this.$store.dispatch(ADD_CREDIT_CARD, token);
        } catch (error) {
            await this.$notify.error(error.message);
            this.setDefaultState();

            return;
        }

        await this.$notify.success('Card successfully added');
        try {
            await this.$store.dispatch(GET_CREDIT_CARDS);
        } catch (error) {
            await this.$notify.error(error.message);
            this.setDefaultState();
        }
    }

    /**
     * Sets area to default state.
     */
    private setDefaultState(): void {
        this.isLoading = false;
        this.$emit(this.TOGGLE_IS_LOADING);
    }
}
</script>

<style scoped lang="scss">
    .add-card-state {
        font-family: 'font_regular', sans-serif;
        width: 100%;
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: space-between;
        margin-bottom: 20px;

        &__input-container {
            position: relative;
            width: calc(100% - 90px);
            padding: 25px 45px 35px 45px;
            background-color: #fff;

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

        &__security-info {
            width: calc(100% - 70px);
            padding: 15px 35px;
            background-color: #cef0e3;
            border-radius: 0 0 8px 8px;
            display: flex;
            align-items: center;
            justify-content: center;

            &__text {
                margin-left: 5px;
                font-size: 15px;
                line-height: 18px;
                color: #1a9666;
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

    .loading {
        opacity: 0.6;
        pointer-events: none;
    }
</style>