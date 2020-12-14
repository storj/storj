// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./overviewStep.html"></template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VButton from '@/components/common/VButton.vue';

import { RouteConfig } from '@/router';

@Component({
    components: {
        VButton,
    },
})
export default class OverviewStep extends Vue {
    /**
     * Holds button click logic.
     * Redirects to next step (adding payment method).
     */
    public onAddPaymentClick(): void {
        this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.PaymentStep).path);
    }

    /**
     * Holds button click logic.
     * Redirects to next step (creating access grant).
     */
    public onCreateGrantClick(): void {
        this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.AccessGrant).path);
    }

    /**
     * Indicates if paywall is enabled.
     */
    public get isPaywallEnabled(): boolean {
        return this.$store.state.paymentsModule.isPaywallEnabled;
    }
}
</script>

<style scoped lang="scss" src="./overviewStep.scss"></style>
