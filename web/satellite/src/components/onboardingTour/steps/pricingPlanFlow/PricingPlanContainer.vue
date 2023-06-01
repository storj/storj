// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="pricing-container">
        <p v-if="isPartner" class="pricing-container__tag">Best Value</p>
        <div class="pricing-container__inner" :class="{partner: isPartner}">
            <div class="pricing-container__inner__section">
                <div class="pricing-container__inner__section__header">
                    <div class="pricing-container__inner__section__header__icon">
                        <CloudIcon v-if="isPartner" />
                        <StarIcon v-else-if="isPro" />
                        <GlobeIcon v-else />
                    </div>
                    <div>
                        <h2 class="pricing-container__inner__section__header__title">{{ plan.title }}</h2>
                        <p>{{ plan.containerSubtitle }}</p>
                    </div>
                </div>
                <p class="pricing-container__inner__section__description">{{ plan.containerDescription }}</p>
                <!-- eslint-disable-next-line vue/no-v-html -->
                <p v-if="plan.containerFooterHTML" class="pricing-container__inner__section__footer" v-html="plan.containerFooterHTML" />
            </div>
            <div class="pricing-container__inner__section">
                <VButton
                    class="pricing-container__inner__section__button"
                    :label="plan.activationButtonText || ('Activate ' + plan.title)"
                    font-size="13px"
                    :on-press="onActivateClick"
                    :is-green="isPartner"
                    :is-white="isFree"
                >
                    <template #icon-right>
                        <ArrowIcon :class="{'arrow-dark': isFree}" />
                    </template>
                </VButton>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { PricingPlanInfo, PricingPlanType } from '@/types/common';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useAppStore } from '@/store/modules/appStore';

import VButton from '@/components/common/VButton.vue';

import ArrowIcon from '@/../static/images/onboardingTour/arrowRight.svg';
import CloudIcon from '@/../static/images/onboardingTour/cloudIcon.svg';
import GlobeIcon from '@/../static/images/onboardingTour/globeIcon.svg';
import StarIcon from '@/../static/images/onboardingTour/starIcon.svg';

const props = defineProps<{
    plan: PricingPlanInfo;
}>();

const appStore = useAppStore();

/**
 * Sets the selected pricing plan and displays the pricing plan modal.
 */
function onActivateClick(): void {
    appStore.setPricingPlan(props.plan);
    appStore.updateActiveModal(MODALS.pricingPlan);
}

const isPartner = computed((): boolean => props.plan.type === PricingPlanType.PARTNER);
const isPro = computed((): boolean => props.plan.type === PricingPlanType.PRO);
const isFree = computed((): boolean => props.plan.type === PricingPlanType.FREE);
</script>

<style scoped lang="scss">
.pricing-container {
    width: 270px;
    min-height: 324px;
    position: relative;
    display: flex;
    flex-direction: column;
    align-items: center;

    &__tag {
        position: absolute;
        transform: translateY(-50%);
        padding: 3px 8px;
        box-sizing: border-box;
        background-color: white;
        border: 1px solid var(--c-green-5);
        border-radius: 24px;
        color: var(--c-green-5);
        font-family: 'font_medium', sans-serif;
        font-size: 12px;
        line-height: 18px;
        text-transform: uppercase;
    }

    &__inner {
        width: 100%;
        height: 100%;
        padding: 32px;
        box-sizing: border-box;
        display: flex;
        flex-direction: column;
        justify-content: space-between;
        gap: 10px;
        background-color: #fff;
        border: 1px solid var(--c-grey-3);
        border-radius: 10px;
        box-shadow: 0 0 20px rgb(0 0 0 / 4%);
        font-family: 'font_regular', sans-serif;
        line-height: 20px;
        text-align: center;

        &.partner {
            border: 2px solid var(--c-green-5);
            box-shadow: 0 7px 20px rgb(0 0 0 / 15%);
        }

        &__section {
            display: flex;
            flex-direction: column;
            align-items: center;
            gap: 10px;

            &__header {
                display: flex;
                flex-direction: column;
                align-items: center;
                gap: 10px;

                &__icon {
                    min-width: 40px;
                    height: 40px;
                    box-sizing: border-box;
                    display: flex;
                    align-items: center;
                    justify-content: center;
                    border: 1px solid var(--c-grey-2);
                    border-radius: 10px;
                }

                &__title {
                    font-family: 'font_bold', sans-serif;
                    font-size: 14px;
                }
            }

            &__description {
                margin-bottom: 20px;
            }

            &__footer {
                line-height: 22px;

                :deep(b) {
                    font-family: 'font_bold', sans-serif;
                }
            }

            &__button {
                padding: 10px 16px;

                & .arrow-dark :deep(path) {
                    fill: var(--c-grey-6);
                }

                &:hover {

                    :deep(path) {
                        stroke: none !important;
                    }

                    &.arrow-dark :deep(path) {
                        fill: white;
                    }
                }
            }
        }
    }
}

@media screen and (width <= 963px) {

    .pricing-container {
        width: 100%;
        min-height: unset;

        &__inner {
            padding: 21px 24px;
            text-align: left;

            &__section {

                &__header {
                    width: 100%;
                    flex-direction: row;
                    margin-bottom: 0;
                }

                &__description {
                    width: 100%;
                }

                &__price {
                    text-align: center;
                }

                &__button {
                    width: 100% !important;
                }
            }
        }
    }
}
</style>
