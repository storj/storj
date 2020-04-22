// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="tour-area">
        <ProgressBar/>
        <OverviewStep
            v-if="isDefaultState"
            @setAddPaymentState="setAddPaymentState"
        />
        <AddPaymentStep
            v-if="isAddPaymentState"
        />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import ProgressBar from '@/components/onboardingTour/ProgressBar.vue';
import AddPaymentStep from '@/components/onboardingTour/steps/AddPaymentStep.vue';
import OverviewStep from '@/components/onboardingTour/steps/OverviewStep.vue';

import CheckedImage from '@/../static/images/common/checked.svg';

import { RouteConfig } from '@/router';
import { TourState } from '@/utils/constants/onboardingTourEnums';

@Component({
    components: {
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
        if (this.$store.state.projectsModule.projects.length > 0) {
            try {
                this.$router.push(RouteConfig.ProjectDashboard.path);
            } catch (error) {
                return;
            }

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
     * Indicates if area is adding payment method state.
     */
    public get isAddPaymentState(): boolean {
        return this.areaState === TourState.ADDING_PAYMENT;
    }

    /**
     * Sets area's state to adding payment method state.
     */
    public setAddPaymentState(): void {
        this.areaState = TourState.ADDING_PAYMENT;
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
