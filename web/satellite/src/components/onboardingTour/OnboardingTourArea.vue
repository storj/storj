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
            <router-view/>
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

import CloseImage from '@/../static/images/onboardingTour/close.svg';

import { RouteConfig } from '@/router';

@Component({
    components: {
        CloseImage,
    },
})
export default class OnboardingTourArea extends Vue {
    public isInfoBarVisible: boolean = true;

    /**
     * Lifecycle hook after initial render.
     * Sets area to needed state.
     */
    public mounted(): void {
        if (this.userHasProject && this.userHasAccessGrants) {
            try {
                this.$router.push(RouteConfig.ProjectDashboard.path);
            } catch (error) {
                return;
            }

            return;
        }

        if (this.$route.name === RouteConfig.AccessGrant.name) {
            this.disableInfoBar();
        }
    }

    /**
     * Indicates if paywall is enabled.
     */
    public get isPaywallEnabled(): boolean {
        return this.$store.state.paymentsModule.isPaywallEnabled;
    }

    /**
     * Sets area's state to creating access grant step.
     */
    public setCreateAccessGrantStep(): void {
        this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.AccessGrant).path);
    }

    /**
     * Disables info bar visibility.
     */
    public disableInfoBar(): void {
        this.isInfoBarVisible = false;
    }

    /**
     * Indicates if area is on adding payment method step.
     */
    public get isAddPaymentState(): boolean {
        return this.$route.name === RouteConfig.PaymentStep.name;
    }

    /**
     * Indicates if user has at least one project.
     */
    private get userHasProject(): boolean {
        return this.$store.state.projectsModule.projects.length > 0;
    }

    /**
     * Indicates if user has at least one access grant.
     */
    private get userHasAccessGrants(): boolean {
        return this.$store.state.accessGrantsModule.page.accessGrants.length > 0;
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
            display: flex;
            flex-direction: column;
            align-items: center;
            padding: 110px 0 80px 0;
            position: relative;

            &__tardigrade {
                position: absolute;
                left: 50%;
                bottom: 0;
                transform: translate(-50%);
            }
        }
    }
</style>
