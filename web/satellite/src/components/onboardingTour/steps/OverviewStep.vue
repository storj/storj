// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./overviewStep.html"></template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VButton from '@/components/common/VButton.vue';

import { RouteConfig } from '@/router';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { ProjectFields } from '@/types/projects';
import { PM_ACTIONS } from '@/utils/constants/actionNames';
import { SegmentEvent } from '@/utils/constants/analyticsEventNames';

@Component({
    components: {
        VButton,
    },
})
export default class OverviewStep extends Vue {
    public isLoading: boolean = false;

    /**
     * Lifecycle hook after initial render.
     * Sets area to needed state.
     */
    public mounted(): void {
        if (this.userHasProject || this.$store.state.paymentsModule.creditCards.length > 0) {
            this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.AccessGrant).path);

            return;
        }

        if (this.$store.getters.isTransactionProcessing || this.$store.getters.isBalancePositive) {
            this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.PaymentStep).path);
        }
    }

    /**
     * Holds button click logic.
     * Redirects to next step (adding payment method).
     */
    public onAddPaymentClick(): void {
        this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.PaymentStep).path);
    }

    /**
     * Holds button click logic.
     * Creates untitled project and redirects to next step (creating access grant).
     */
    public async onCreateGrantClick(): Promise<void> {
        if (this.isLoading) return;

        this.isLoading = true;

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

            this.isLoading = false;

            await this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.AccessGrant).with(RouteConfig.AccessGrantName).path);
        } catch (error) {
            await this.$notify.error(error.message);
            this.isLoading = false;
        }
    }

    /**
     * Indicates if paywall is enabled.
     */
    public get isPaywallEnabled(): boolean {
        return this.$store.state.paymentsModule.isPaywallEnabled;
    }

    /**
     * Indicates if user has at least one project.
     */
    private get userHasProject(): boolean {
        return this.$store.state.projectsModule.projects.length > 0;
    }
}
</script>

<style scoped lang="scss" src="./overviewStep.scss"></style>
