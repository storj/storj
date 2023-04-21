// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div v-if="isAddModal" class="add-modal">
                <div class="add-modal__top">
                    <h1 class="add-modal__top__title" aria-roledescription="modal-title">Upgrade to Pro Account</h1>
                    <div class="add-modal__top__header">
                        <p class="add-modal__top__header__sub-title">Add Payment Method</p>
                        <div class="add-modal__top__header__choices">
                            <p class="add-modal__top__header__choices__var" :class="{active: !isAddCard}" @click.stop="setIsAddToken">
                                STORJ Token
                            </p>
                            <p class="add-modal__top__header__choices__var" :class="{active: isAddCard}" @click.stop="setIsAddCard">
                                Card
                            </p>
                        </div>
                    </div>
                </div>
                <div v-if="isAddCard" class="add-modal__card">
                    <StripeCardInput
                        ref="stripeCardInput"
                        class="add-modal__card__stripe"
                        :on-stripe-response-callback="addCardToDB"
                    />
                    <VButton
                        width="100%"
                        height="48px"
                        border-radius="32px"
                        label="Add Credit Card"
                        :on-press="onAddCardClick"
                    />
                    <p class="add-modal__card__info">Pay as you go, no contract required.</p>
                </div>
                <div v-else class="add-modal__tokens">
                    <p class="add-modal__tokens__banner">
                        Deposit STORJ Token to your account and receive a 10% bonus, or $10 for every $100.
                    </p>
                    <p class="add-modal__tokens__support-info">To deposit STORJ token and request higher limits, please contact <a target="_blank" rel="noopener noreferrer" href="https://supportdcs.storj.io/hc/en-us/requests/new?ticket_form_id=360000683212">Support</a></p>
                </div>
                <div class="add-modal__bullets">
                    <div class="add-modal__bullets__left">
                        <h2 class="add-modal__bullets__left__title">Pro Account Limits:</h2>
                        <div class="add-modal__bullets__left__item">
                            <CheckMarkIcon />
                            <p class="add-modal__bullets__left__item__label">3 projects</p>
                        </div>
                        <div class="add-modal__bullets__left__item">
                            <CheckMarkIcon />
                            <p class="add-modal__bullets__left__item__label">100 buckets per project</p>
                        </div>
                        <div class="add-modal__bullets__left__item">
                            <CheckMarkIcon />
                            <p class="add-modal__bullets__left__item__label">Up to 25 TB storage per project</p>
                        </div>
                        <div class="add-modal__bullets__left__item">
                            <CheckMarkIcon />
                            <p class="add-modal__bullets__left__item__label">Up to 100 TB egress bandwidth per project per month</p>
                        </div>
                        <div class="add-modal__bullets__left__item">
                            <CheckMarkIcon />
                            <p class="add-modal__bullets__left__item__label">100 request per second rate limit</p>
                        </div>
                    </div>
                    <VLoader v-if="isPriceFetching" class="add-modal__bullets__right-loader" width="90px" height="90px" />
                    <div v-else class="add-modal__bullets__right">
                        <h2 class="add-modal__bullets__right__title">Storage price:</h2>
                        <div class="add-modal__bullets__right__item">
                            <p class="add-modal__bullets__right__item__price">{{ storagePrice }}</p>
                            <p class="add-modal__bullets__right__item__label">TB / month</p>
                        </div>
                        <h2 class="add-modal__bullets__right__title">Bandwidth price:</h2>
                        <div class="add-modal__bullets__right__item">
                            <p class="add-modal__bullets__right__item__price">{{ bandwidthPrice }}</p>
                            <p class="add-modal__bullets__right__item__label">TB</p>
                        </div>
                        <!-- eslint-disable-next-line vue/no-v-html -->
                        <p v-if="extraBandwidthPriceInfo" class="add-modal__bullets__right__item__label__special" v-html="extraBandwidthPriceInfo" />
                    </div>
                </div>
                <div class="add-modal__security">
                    <LockImage />
                    <p class="add-modal__security__info">
                        Your information is secured with 128-bit SSL & AES-256 encryption.
                    </p>
                </div>
                <div v-if="isLoading" class="add-modal__blur">
                    <VLoader
                        class="add-modal__blur__loader"
                        width="30px"
                        height="30px"
                    />
                </div>
            </div>
            <div v-else class="success-modal">
                <BigCheckMarkIcon />
                <h2 class="success-modal__title">Congratulations!</h2>
                <h2 class="success-modal__sub-title">You've just upgraded to a Pro Account.</h2>
                <p class="success-modal__info">
                    Now you can have up to
                    <b class="success-modal__info__bold">75TB</b>
                    of total storage and
                    <b>300TB</b>
                    of egress bandwidth per month. If you need more
                    than this, please
                    <a
                        class="success-modal__info__link"
                        :href="limitsIncreaseRequestURL"
                        target="_blank"
                        rel="noopener noreferrer"
                    >
                        contact us
                    </a>
                    .
                </p>
                <VButton
                    width="100%"
                    height="48px"
                    border-radius="32px"
                    label="Done"
                    :on-press="closeModal"
                />
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { computed, onMounted, onBeforeMount, ref, reactive } from 'vue';

import { useNotify, useRouter } from '@/utils/hooks';
import { RouteConfig } from '@/router';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { ProjectUsagePriceModel } from '@/types/payments';
import { decimalShift, formatPrice, CENTS_MB_TO_DOLLARS_TB_SHIFT } from '@/utils/strings';
import { useUsersStore } from '@/store/modules/usersStore';
import { useBillingStore } from '@/store/modules/billingStore';
import { useAppStore } from '@/store/modules/appStore';
import { useProjectsStore } from '@/store/modules/projectsStore';

import VModal from '@/components/common/VModal.vue';
import VLoader from '@/components/common/VLoader.vue';
import VButton from '@/components/common/VButton.vue';
import StripeCardInput from '@/components/account/billing/paymentMethods/StripeCardInput.vue';

import BigCheckMarkIcon from '@/../static/images/common/greenRoundCheckmarkBig.svg';
import CheckMarkIcon from '@/../static/images/common/greenRoundCheckmark.svg';
import LockImage from '@/../static/images/account/billing/greyLock.svg';

interface StripeForm {
    onSubmit(): Promise<void>;
}

const appStore = useAppStore();
const billingStore = useBillingStore();
const usersStore = useUsersStore();
const projectsStore = useProjectsStore();
const notify = useNotify();
const nativeRouter = useRouter();
const router = reactive(nativeRouter);

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

const isAddModal = ref<boolean>(true);
const isAddCard = ref<boolean>(true);
const isLoading = ref<boolean>(false);
const isPriceFetching = ref<boolean>(true);

const stripeCardInput = ref<typeof StripeCardInput & StripeForm | null>(null);

const extraBandwidthPriceInfo = ref<string>('');

/**
 * Lifecycle hook after initial render.
 * Fetches project usage price model.
 */
onMounted(async () => {
    try {
        await billingStore.getProjectUsagePriceModel();
        isPriceFetching.value = false;
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.UPGRADE_ACCOUNT_MODAL);
    }
});

/**
 * Provides card information to Stripe.
 */
async function onAddCardClick(): Promise<void> {
    if (isLoading.value || !stripeCardInput.value) return;

    isLoading.value = true;

    try {
        await stripeCardInput.value.onSubmit();
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.UPGRADE_ACCOUNT_MODAL);
    }
    isLoading.value = false;
}

/**
 * Adds card after Stripe confirmation.
 *
 * @param token from Stripe
 */
async function addCardToDB(token: string): Promise<void> {
    try {
        await billingStore.addCreditCard(token);
        await notify.success('Card successfully added');
        // We fetch User one more time to update their Paid Tier status.
        await usersStore.getUser();

        if (router.currentRoute.name === RouteConfig.ProjectDashboard.name) {
            await projectsStore.getProjectLimits(projectsStore.state.selectedProject.id);
        }

        await analytics.eventTriggered(AnalyticsEvent.MODAL_ADD_CARD);

    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.UPGRADE_ACCOUNT_MODAL);
    }

    isLoading.value = false;
    isAddModal.value = false;
}

/**
 * Closes add payment method modal.
 */
function closeModal(): void {
    appStore.updateActiveModal(MODALS.addPaymentMethod);
}

/**
 * Sets modal state to add STORJ tokens.
 */
function setIsAddToken(): void {
    isAddCard.value = false;
}

/**
 * Sets modal state to add credit card.
 */
function setIsAddCard(): void {
    isAddCard.value = true;
}

/**
 * Returns project limits increase request url from config.
 */
const limitsIncreaseRequestURL = computed((): string => {
    return appStore.state.config.projectLimitsIncreaseRequestURL;
});

/**
 * Returns project usage price model from store.
 */
const priceModel = computed((): ProjectUsagePriceModel => {
    return billingStore.state.usagePriceModel;
});

/**
 * Returns the storage price formatted as dollars per terabyte.
 */
const storagePrice = computed((): string => {
    const storage = priceModel.value.storageMBMonthCents.toString();
    return formatPrice(decimalShift(storage, CENTS_MB_TO_DOLLARS_TB_SHIFT));
});

/**
 * Returns the bandwidth (egress) price formatted as dollars per terabyte.
 */
const bandwidthPrice = computed((): string => {
    const egress = priceModel.value.egressMBCents.toString();
    return formatPrice(decimalShift(egress, CENTS_MB_TO_DOLLARS_TB_SHIFT));
});

/**
 * Lifecycle hook before initial render.
 * If applicable, loads additional clarifying text based on user partner.
 */
onBeforeMount(() => {
    try {
        const partner = usersStore.state.user.partner;
        const config = require('@/components/account/billing/billingConfig.json');
        if (partner !== '' && config[partner] && config[partner].extraBandwidthPriceInfo) {
            extraBandwidthPriceInfo.value = config[partner].extraBandwidthPriceInfo;
        }
    } catch (e) {
        notify.error('No configuration file for page.', null);
    }
});
</script>

<style scoped lang="scss">
    .add-modal {
        width: 760px;
        padding-top: 50px;
        font-family: 'font_regular', sans-serif;

        @media screen and (max-width: 850px) {
            width: unset;
        }

        &__top {
            padding: 0 50px;

            @media screen and (max-width: 850px) {
                padding: 0 36px;
            }

            @media screen and (max-width: 570px) {
                padding: 0 24px;
            }

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 36px;
                line-height: 44px;
                color: #1b2533;
                margin-bottom: 40px;
                text-align: left;

                @media screen and (max-width: 420px) {
                    max-width: 248px;
                }
            }

            &__header {
                display: flex;
                align-items: center;
                justify-content: space-between;
                margin-bottom: 30px;

                @media screen and (max-width: 570px) {
                    flex-direction: column;
                    align-items: flex-start;
                    justify-content: unset;
                }

                &__sub-title {
                    font-size: 18px;
                    line-height: 22px;
                    color: #000;
                }

                &__choices {
                    display: flex;
                    align-items: center;
                    column-gap: 20px;

                    @media screen and (max-width: 570px) {
                        margin-top: 23px;
                        column-gap: 50px;
                    }

                    &__var {
                        font-family: 'font_medium', sans-serif;
                        font-weight: 600;
                        font-size: 14px;
                        line-height: 18px;
                        color: var(--c-blue-3);
                        padding: 0 10px 5px;
                        cursor: pointer;
                        border-bottom: 3px solid #fff;

                        @media screen and (max-width: 570px) {
                            padding: 0 0 5px;
                        }
                    }
                }
            }
        }

        &__card {
            padding: 0 50px;
            margin-bottom: 20px;

            @media screen and (max-width: 850px) {
                padding: 0 36px;
                width: 642px;
            }

            @media screen and (max-width: 767px) {
                width: unset;
            }

            @media screen and (max-width: 570px) {
                padding: 0 24px;
            }

            &__stripe {
                margin: 20px 0;
            }

            &__info {
                margin-top: 20px;
                font-size: 12px;
                line-height: 19px;
                text-align: center;
                color: #a8a8a8;
            }
        }

        &__tokens {
            padding: 0 50px;
            margin-bottom: 30px;

            @media screen and (max-width: 850px) {
                padding: 0 36px;
            }

            @media screen and (max-width: 570px) {
                padding: 0 24px;
            }

            &__banner {
                font-size: 14px;
                line-height: 20px;
                color: #384761;
                padding: 20px 35px;
                background: #edf4fe;
                border-radius: 8px;
                margin-bottom: 25px;

                @media screen and (max-width: 570px) {
                    padding: 16px 20px;
                }
            }

            &__selection {
                margin-bottom: 25px;
            }

            &__checkout-container {
                display: flex;
                justify-content: center;
                margin-top: 25px;

                &__link {
                    font-size: 16px;
                    line-height: 20px;
                    color: #2683ff;
                }
            }

            &__note {
                font-size: 14px;
                line-height: 20px;
                color: #14142a;
                margin: 25px 0;
                text-align: left;
            }

            &__info {
                font-size: 14px;
                line-height: 20px;
                color: #14142a;
                text-align: left;

                &__link {
                    font-family: 'font_medium', sans-serif;
                    text-decoration: underline !important;
                    text-underline-position: under;

                    &:visited {
                        color: #14142a;
                    }
                }
            }

            &__support-info {
                font-weight: 600;
                font-size: 14px;
                line-height: 20px;
                color: #000;

                a {
                    color: var(--c-blue-3);
                }
            }
        }

        &__bullets {
            background: #f0f0f0;
            padding: 35px 50px 90px;
            border-radius: 0 0 32px 32px;
            display: flex;

            @media screen and (max-width: 850px) {
                padding: 35px 36px 90px;
            }

            @media screen and (max-width: 570px) {
                padding: 35px 24px 90px;
                flex-direction: column;
            }

            &__left {
                width: 50%;
                border-right: 1px solid #ccc;

                @media screen and (max-width: 570px) {
                    width: 100%;
                    border-right: unset;
                }

                &__title {
                    font-family: 'font_medium', sans-serif;
                    font-size: 16px;
                    line-height: 26px;
                    color: #000;
                    margin-bottom: 5px;
                    text-align: left;
                }

                &__item {
                    display: flex;
                    align-items: center;
                    margin-top: 12px;

                    svg {
                        min-width: 20px;
                    }

                    &__label {
                        margin-left: 12px;
                        font-size: 14px;
                        line-height: 20px;
                        letter-spacing: 0.4735px;
                        color: #000;
                        text-align: left;
                    }
                }
            }

            &__right-loader {
                width: 50%;
                align-items: center;

                @media screen and (max-width: 570px) {
                    width: 100%;
                    margin-top: 16px;
                }
            }

            &__right {
                padding-left: 50px;

                @media screen and (max-width: 570px) {
                    padding-left: unset;
                    margin-top: 35px;
                }

                &__title {
                    font-family: 'font_medium', sans-serif;
                    font-size: 16px;
                    line-height: 26px;
                    color: #000;
                    margin-bottom: 5px;
                    text-align: left;

                    &:last-of-type {
                        margin-top: 25px;
                    }
                }

                &__item {
                    display: flex;
                    align-items: flex-start;
                    letter-spacing: 0.4735px;

                    &__price {
                        font-family: 'font_bold', sans-serif;
                        font-size: 42px;
                        color: var(--c-blue-3);
                    }

                    &__label {
                        font-family: 'font_medium', sans-serif;
                        font-size: 16px;
                        line-height: 20px;
                        color: #a9a9a9;
                        margin: 5px 0 0 5px;

                        &__special {
                            font-size: 13px;
                            text-align: left;
                            color: #000;
                        }
                    }
                }
            }
        }

        &__security {
            display: flex;
            align-items: center;
            justify-content: center;
            position: absolute;
            bottom: 0;
            left: 0;
            right: 0;
            background: #fff;
            border-radius: 0 0 32px 32px;
            padding: 15px 36px;

            @media screen and (max-width: 570px) {
                padding: 15px 20px;
            }

            svg {
                min-width: 20px;
            }

            &__info {
                font-weight: 500;
                font-size: 15px;
                line-height: 18px;
                color: #3f3f3f;
                margin-left: 12px;
                text-align: left;
            }
        }

        &__blur {
            position: absolute;
            top: 0;
            bottom: 0;
            left: 0;
            right: 0;
            border-radius: 32px;
            z-index: 1;
            background-color: rgb(245 246 250 / 50%);
            display: flex;
            align-items: center;
            justify-content: center;
        }
    }

    .success-modal {
        width: 480px;
        padding: 50px;
        font-family: 'font_regular', sans-serif;

        &__title,
        &__sub-title {
            font-size: 36px;
            line-height: 54px;
            color: #000;
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            margin-top: 20px;
        }

        &__sub-title {
            margin-top: 15px;
        }

        &__info {
            margin: 35px 0 48px;
            font-size: 18px;
            line-height: 32px;
            color: #000;

            &__bold {
                font-family: 'font_bold', sans-serif;
            }

            &__link {
                font-family: 'font_bold', sans-serif;
                text-decoration: underline !important;
                text-underline-position: under;

                &:visited {
                    color: #000;
                }
            }
        }
    }

    .active {
        border-color: var(--c-blue-3);
    }
</style>
