// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal v-if="plan" class="modal" :on-close="onClose">
        <template #content>
            <div v-if="!isSuccess" class="content">
                <div class="content__top">
                    <h1 class="content__top__title">Activate your plan</h1>
                    <div class="content__top__icon">
                        <CheckIcon />
                    </div>
                </div>
                <div class="content__middle">
                    <div>
                        <p class="content__middle__name">
                            {{ plan.title }}
                            <span v-if="plan.activationSubtitle"> / {{ plan.activationSubtitle }}</span>
                        </p>
                        <!-- eslint-disable-next-line vue/no-v-html -->
                        <p class="content__middle__description" v-html="plan.activationDescriptionHTML" />
                    </div>
                    <!-- eslint-disable-next-line vue/no-v-html -->
                    <p v-if="plan.activationPriceHTML" class="content__middle__price" v-html="plan.activationPriceHTML" />
                </div>
                <div class="content__bottom">
                    <div v-if="!isFree" class="content__bottom__card-area">
                        <p class="content__bottom__card-area__label">Add Card Info</p>
                        <StripeCardElement
                            v-if="paymentElementEnabled"
                            ref="stripeCardInput"
                            class="content__bottom__card-area__input"
                            @pm-created="onCardAdded"
                        />
                        <StripeCardInput
                            v-else
                            ref="stripeCardInput"
                            :on-stripe-response-callback="onCardAdded"
                        />
                    </div>
                    <VButton
                        class="content__bottom__button"
                        :label="plan.activationButtonText || ('Activate ' + plan.title)"
                        width="100%"
                        font-size="13px"
                        icon="lock"
                        :is-green="plan.type === 'partner'"
                        :is-disabled="isLoading"
                        :on-press="onActivateClick"
                    />
                    <VButton
                        class="content__bottom__button"
                        label="Cancel"
                        width="100%"
                        font-size="13px"
                        :is-white="true"
                        :on-press="onClose"
                    />
                </div>
            </div>
            <div v-else class="content-success">
                <div class="content-success__icon">
                    <CircleCheck />
                </div>
                <h1 class="content-success__title">Success</h1>
                <p class="content-success__subtitle">Your plan has been successfully activated.</p>
                <div class="content-success__info">
                    <ThinCheck class="content-success__info__icon" />
                    <p class="content-success__info__title">
                        {{ plan.title }}
                        <span v-if="plan.activationSubtitle" class="content-success__info__title__duration"> / {{ plan.successSubtitle }}</span>
                    </p>
                    <p class="content-success__info__activated">Activated</p>
                </div>
                <VButton
                    class="content-success__button"
                    label="Continue"
                    font-size="13px"
                    :on-press="onClose"
                />
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useRouter } from 'vue-router';

import { RouteConfig } from '@/types/router';
import { PricingPlanInfo, PricingPlanType } from '@/types/common';
import { useNotify } from '@/utils/hooks';
import { useUsersStore } from '@/store/modules/usersStore';
import { useBillingStore } from '@/store/modules/billingStore';
import { useAppStore } from '@/store/modules/appStore';
import { useConfigStore } from '@/store/modules/configStore';

import VButton from '@/components/common/VButton.vue';
import VModal from '@/components/common/VModal.vue';
import StripeCardElement from '@/components/account/billing/paymentMethods/StripeCardElement.vue';
import StripeCardInput from '@/components/account/billing/paymentMethods/StripeCardInput.vue';

import CheckIcon from '@/../static/images/common/check.svg';
import CircleCheck from '@/../static/images/onboardingTour/circleCheck.svg';
import ThinCheck from '@/../static/images/onboardingTour/thinCheck.svg';

interface StripeForm {
    onSubmit(): Promise<void>;
}

const configStore = useConfigStore();
const appStore = useAppStore();
const billingStore = useBillingStore();
const usersStore = useUsersStore();
const router = useRouter();
const notify = useNotify();

const isLoading = ref<boolean>(false);
const isSuccess = ref<boolean>(false);

const stripeCardInput = ref<StripeForm | null>(null);

/**
 * Returns the pricing plan selected from the onboarding tour.
 */
const plan = computed((): PricingPlanInfo | null => {
    return appStore.state.selectedPricingPlan;
});

/**
 * Indicates whether stripe payment element is enabled.
 */
const paymentElementEnabled = computed(() => {
    return configStore.state.config.stripePaymentElementEnabled;
});

watch(plan, () => {
    if (!plan.value) {
        appStore.removeActiveModal();
        notify.error('No pricing plan has been selected.');
    }
});

/**
 * Returns whether this modal corresponds to a free pricing plan.
 */
const isFree = computed((): boolean => {
    return plan.value?.type === PricingPlanType.FREE;
});

/**
 * Closes the modal. If the user has not completed the onboarding tour, advance to the next step.
 */
function onClose(): void {
    appStore.removeActiveModal();
    // do not reroute if the user has already completed onboarding
    if (usersStore.state.settings.onboardingEnd) {
        return;
    }

    if (isSuccess.value) router.push(RouteConfig.AllProjectsDashboard.path);
}

/**
 * Applies the selected pricing plan to the user.
 */
function onActivateClick(): void {
    if (isLoading.value || !plan.value) return;
    isLoading.value = true;

    if (isFree.value) {
        isSuccess.value = true;
        return;
    }

    stripeCardInput.value?.onSubmit();
}

/**
 * Adds card after Stripe confirmation.
 * @param res - the response from stripe. Could be a token or a payment method id.
 * depending on the paymentElementEnabled flag.
 */
async function onCardAdded(res: string): Promise<void> {
    if (!plan.value) return;

    try {
        if (plan.value.type === PricingPlanType.PARTNER) {
            await billingStore.purchasePricingPackage(res, paymentElementEnabled.value);
        } else {
            paymentElementEnabled.value ? await billingStore.addCardByPaymentMethodID(res) : await billingStore.addCreditCard(res);
        }
        isSuccess.value = true;

        // Fetch user to update paid tier status
        await usersStore.getUser();
        // Fetch cards to hide paid tier banner
        await billingStore.getCreditCards();
    } catch (error) {
        notify.notifyError(error);
    }

    isLoading.value = false;
}
</script>

<style scoped lang="scss">
.modal {

    :has(.content) :deep(.mask__wrapper__container) {
        background-color: var(--c-grey-1);
    }

    :has(.content-success) :deep(.mask__wrapper__container) {
        background-color: var(--c-grey-2);
    }

    :deep(.mask__wrapper__container__close) {
        display: none;
    }
}

.content {
    width: 444px;
    font-family: 'font_regular', sans-serif;
    line-height: 20px;
    text-align: left;

    &__top {
        padding: 26px;
        display: flex;
        flex-direction: row;
        justify-content: space-between;
        align-items: center;
        gap: 16px;
        border-radius: 10px 10px 0 0;

        &__title {
            font-size: 14px;
            font-family: 'font_bold', sans-serif;
        }

        &__icon {
            width: 40px;
            height: 40px;
            display: flex;
            align-items: center;
            justify-content: center;
            background-color: var(--c-green-4);
            border-radius: 10px;

            svg {
                transform: scale(1.25);

                :deep(path) {
                    fill: var(--c-green-5);
                }
            }
        }
    }

    &__middle {
        padding: 36px 26px;
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 16px;
        background-color: #fff;

        &__name {
            font-family: 'font_bold', sans-serif;
        }

        &__description :deep(a) {
            color: var(--c-blue-3);
            text-decoration: underline;
        }

        &__price {
            font-family: 'font_medium', sans-serif;
            line-height: 22px;

            :deep(s) {
                font-family: 'font_regular', sans-serif;
            }

            :deep(b) {
                font-family: 'font_bold', sans-serif;
            }
        }
    }

    &__bottom {
        padding: 24px;
        border-radius: 0 0 10px 10px;

        > :not(:last-child) {
            margin-bottom: 8px;
        }

        &__card-area {

            &__label {
                margin-bottom: 8px;
                color: var(--c-grey-6);
            }
        }

        &__button {
            padding: 10px;

            :deep(svg path) {
                fill: #fff;
            }
        }
    }
}

.content-success {
    width: 444px;
    padding: 50px 25px;
    display: flex;
    flex-direction: column;
    align-items: center;
    border-radius: 10px;
    font-family: 'font_regular', sans-serif;

    &__icon {
        width: 65px;
        height: 65px;
        display: flex;
        align-items: center;
        justify-content: center;
        margin-bottom: 10px;
        background-color: var(--c-green-5);
        border-radius: 26px;
    }

    &__title {
        margin-bottom: 2px;
        font-family: 'font_bold', sans-serif;
        font-size: 24px;
        line-height: 31px;
        letter-spacing: -0.02em;
        text-align: center;
    }

    &__subtitle {
        margin-bottom: 16px;
        font-weight: 400;
        font-size: 16px;
        line-height: 24px;
        text-align: center;
    }

    &__info {
        width: 100%;
        box-sizing: border-box;
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 16px;
        margin-bottom: 25px;
        padding: 18px 20px;
        background-color: #fff;
        border: 1px solid var(--c-green-5);
        border-radius: 10px;
        box-shadow: 0 0 20px rgb(0 0 0 / 4%);

        &__icon {
            min-width: 20px;
        }

        &__title {
            flex-grow: 1;
            font-family: 'font_bold', sans-serif;
            text-align: left;

            &__duration {
                font-family: 'font_regular', sans-serif;
            }
        }

        &__activated {
            font-family: 'font_medium', sans-serif;
            color: var(--c-green-5);
        }
    }

    &__button {
        width: 143px;
        padding: 10px;
    }
}
</style>
