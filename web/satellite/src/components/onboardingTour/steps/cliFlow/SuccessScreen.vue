// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="success-screen">
        <Icon />
        <h1 class="success-screen__title">Wonderful</h1>
        <p class="success-screen__msg">
            This was easy right :)
            <span v-if="!creditCards.length">
                Wish you all the best with your projects. To upgrade your account and upload up to
                75TB, just add your card or STORJ tokens.
            </span>
        </p>
        <div class="success-screen__buttons">
            <VButton
                class="success-screen__buttons__back"
                label="Back"
                height="64px"
                border-radius="52px"
                is-grey-blue="true"
                :on-press="onBackClick"
            />
            <VButton
                v-if="!creditCards.length"
                class="success-screen__buttons__upgrade"
                label="Upgrade"
                height="64px"
                border-radius="52px"
                :on-press="onUpgradeClick"
            />
            <VButton
                label="Finish"
                height="64px"
                border-radius="52px"
                is-grey-blue="true"
                :on-press="onFinishClick"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { CreditCard } from "@/types/payments";
import { RouteConfig } from "@/router";
import { PAYMENTS_MUTATIONS } from "@/store/modules/payments";

import VButton from "@/components/common/VButton.vue";

import Icon from "@/../static/images/onboardingTour/successStep.svg";

// @vue/component
@Component({
    components: {
        Icon,
        VButton,
    }
})
export default class SuccessScreen extends Vue {
    /**
     * Returns credit cards from store.
     */
    public get creditCards(): CreditCard[] {
        return this.$store.state.paymentsModule.creditCards;
    }

    /**
     * Holds on back button click logic.
     */
    public async onBackClick(): Promise<void> {
        await this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.ShareObject)).path);
    }

    /**
     * Holds on upgrade button click logic.
     */
    public async onUpgradeClick(): Promise<void> {
        await this.$router.push(RouteConfig.ProjectDashboard.path);
        this.$store.commit(PAYMENTS_MUTATIONS.TOGGLE_IS_ADD_PM_MODAL_SHOWN);
    }

    /**
     * Holds on finish button click logic.
     */
    public async onFinishClick(): Promise<void> {
        await this.$router.push(RouteConfig.ProjectDashboard.path);
    }
}
</script>

<style scoped lang="scss">
    .success-screen {
        font-family: 'font_regular', sans-serif;
        background: #fcfcfc;
        box-shadow: 0 0 32px rgba(0, 0, 0, 0.04);
        border-radius: 20px;
        padding: 48px;
        max-width: 484px;

        &__title {
            margin: 20px 0;
            font-family: 'font_Bold', sans-serif;
            font-size: 48px;
            line-height: 56px;
            letter-spacing: 1px;
            color: #14142b;
        }

        &__msg {
            font-size: 18px;
            line-height: 32px;
            letter-spacing: 0.15px;
            color: #4e4b66;
        }

        &__buttons {
            display: flex;
            align-items: center;
            width: 100%;
            margin-top: 48px;

            &__back,
            &__upgrade {
                margin-right: 24px;
            }
        }
    }
</style>
