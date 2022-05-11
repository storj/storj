// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="success-screen">
        <Icon />
        <h1 class="success-screen__title" aria-roledescription="title">Wonderful</h1>
        <p class="success-screen__msg">
            This was easy right? We wish you all the best with your projects.
            <span v-if="!creditCards.length">
                To upgrade your account and upload up to 75TB, just add your card or STORJ tokens.
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
                class="success-screen__buttons__finish"
                label="Finish"
                height="64px"
                border-radius="52px"
                :on-press="onFinishClick"
            />
            <button
                v-if="!creditCards.length"
                class="success-screen__buttons__upgrade"
                type="button"
                @click="onUpgradeClick"
            >
                Upgrade
                <UpgradeIcon class="success-screen__buttons__upgrade__icon" />
            </button>
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
import UpgradeIcon from "@/../static/images/onboardingTour/upgrade.svg";

// @vue/component
@Component({
    components: {
        Icon,
        VButton,
        UpgradeIcon,
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
        box-shadow: 0 0 32px rgb(0 0 0 / 4%);
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
            margin-top: 24px;

            &__back,
            &__finish {
                margin-right: 24px;
            }

            &__upgrade {
                background-color: #fff;
                border: 2px solid #d9dbe9;
                color: #0149ff;
                display: flex;
                align-items: center;
                justify-content: center;
                cursor: pointer;
                font-family: 'font_medium', sans-serif;
                font-size: 16px;
                line-height: 23px;
                white-space: nowrap;
                width: inherit;
                height: 64px;
                border-radius: 52px;

                &__icon {
                    margin-left: 15px;
                }

                &:hover {
                    background-color: #2683ff;
                    border-color: #2683ff;
                    color: #fff;

                    .upgrade-svg-path {
                        stroke: #fff;
                    }
                }
            }
        }
    }
</style>
