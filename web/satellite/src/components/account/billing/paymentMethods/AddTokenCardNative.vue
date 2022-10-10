// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div v-if="wallet.address" class="token">
        <div class="token__icon">
            <div class="token__icon__wrapper">
                <StorjLarge />
            </div>
        </div>
        <div class="token__title-area">
            <div class="token__title-area__small-icon">
                <StorjSmall />
            </div>
            <div class="token__title-area__default-wrapper">
                <p class="token__title-area__default-wrapper__label">Default</p>
                <VInfo>
                    <template #icon>
                        <InfoIcon />
                    </template>
                    <template #message>
                        <p class="token__title-area__default-wrapper__message">
                            If the STORJ token balance runs out, the default credit card will be charged.
                            <a
                                class="token__title-area__default-wrapper__message__link"
                                href=""
                                target="_blank"
                                rel="noopener noreferrer"
                            >
                                Learn More
                            </a>
                        </p>
                    </template>
                </VInfo>
            </div>
        </div>
        <div class="token__info-area">
            <div class="token__info-area__option">
                <h2 class="token__info-area__option__title">STORJ Token Deposit Address</h2>
                <p class="token__info-area__option__value">{{ wallet.address }}</p>
            </div>
            <div class="token__info-area__option">
                <h2 class="token__info-area__option__title">Total Balance</h2>
                <p class="token__info-area__option__value">{{ wallet.balance.value }}</p>
            </div>
        </div>
        <div class="token__action-area">
            <VButton
                class="token__action-area__history-btn"
                label="See transactions"
                is-transparent="true"
                height="32px"
                font-size="13px"
                border-radius="6px"
                :on-press="() => $emit('showTransactions')"
            />

            <v-button
                label="Add funds"
                width="96px"
                height="32px"
                font-size="13px"
                border-radius="6px"
                :on-press="onAddTokensClick"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { Wallet } from '@/types/payments';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';

import VButton from '@/components/common/VButton.vue';
import VInfo from '@/components/common/VInfo.vue';

import InfoIcon from '@/../static/images/billing/blueInfoIcon.svg';
import StorjSmall from '@/../static/images/billing/storj-icon-small.svg';
import StorjLarge from '@/../static/images/billing/storj-icon-large.svg';

// @vue/component
@Component({
    components: {
        InfoIcon,
        StorjSmall,
        StorjLarge,
        VButton,
        VInfo,
    },
})
export default class AddTokenCardNative extends Vue {
    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    /**
     * Holds on add tokens button click logic.
     * Triggers Add funds popup.
     */
    public onAddTokensClick(): void {
        this.analytics.eventTriggered(AnalyticsEvent.ADD_FUNDS_CLICKED);
        this.$store.commit(APP_STATE_MUTATIONS.TOGGLE_ADD_TOKEN_FUNDS_MODAL_SHOWN);
    }

    /**
     * Returns wallet from store.
     */
    private get wallet(): Wallet {
        return this.$store.state.paymentsModule.wallet;
    }
}
</script>

<style scoped lang="scss">
    .token {
        border-radius: 10px;
        width: 300px;
        margin-right: 10px;
        padding: 24px;
        box-shadow: 0 0 20px rgb(0 0 0 / 4%);
        position: relative;
        font-family: 'font_regular', sans-serif;
        background-color: #fff;

        &__icon {
            position: absolute;
            top: 0;
            right: 0;
            width: 120px;
            height: 120px;
            overflow: hidden;

            &__wrapper {
                position: absolute;
                top: -20px;
                right: -20px;
            }
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 18px;
            line-height: 27px;
            color: #000;
            margin-bottom: 5px;
        }

        &__info {
            position: relative;
            font-size: 14px;
            line-height: 20px;
            color: #56606d;
            margin-bottom: 23px;
            max-width: 232px;
            z-index: 1;
        }

        &__title-area {
            position: relative;
            display: flex;
            align-items: center;
            justify-content: space-between;
            z-index: 1;

            &__small-icon {
                background: #e6edf7;
                border-radius: 4px;
                width: 32px;
                height: 24px;
                display: flex;
                align-items: center;
                justify-content: center;
            }

            &__default-wrapper {
                display: flex;
                align-items: center;
                padding: 7px 8px;
                background: #e6edf7;
                border: 1px solid #fff;
                border-radius: 4px;

                &__label {
                    font-family: 'font_bold', sans-serif;
                    font-size: 12px;
                    color: #003dc1;
                    margin-right: 4px;
                }

                &__message {
                    font-size: 12px;
                    line-height: 18px;
                    text-align: center;
                    color: #fff;
                    transform: rotate(180deg);

                    &__link {
                        color: #fff;
                        text-decoration: underline !important;
                        text-underline-position: under;
                    }
                }

                svg {
                    cursor: pointer;
                }
            }
        }

        &__info-area {
            position: relative;
            display: flex;
            align-items: center;
            z-index: 1;
            margin: 32px 0 25px;

            &__option {

                &:first-of-type {
                    margin-right: 16px;
                    max-width: 185px;
                }

                &__title {
                    font-family: 'font_medium', sans-serif;
                    font-size: 12px;
                    line-height: 18px;
                    color: #56606d;
                }

                &__value {
                    font-family: 'font_bold', sans-serif;
                    font-size: 16px;
                    line-height: 24px;
                    color: #000;

                    &:first-of-type {
                        white-space: nowrap;
                        text-overflow: ellipsis;
                        overflow: hidden;
                    }
                }
            }
        }

        &__action-area {
            display: flex;
            justify-content: start;
            align-items: center;
            gap: 10px;

            &__history-btn {
                cursor: pointer;
                padding: 0 10px;

                span {
                    font-size: 13px;
                    color: #56606d;
                    font-family: 'font_medium', sans-serif;
                    line-height: 23px;
                    margin: 0;
                    white-space: nowrap;
                }
            }
        }
    }

    :deep(.info__box) {
        transform: rotate(-180deg);
        top: calc(100% - 100px);
        left: calc(100% - 123px);
        filter: none;
    }

    :deep(.info__box__message) {
        padding: 8px 8px 13px;
        width: 235px;
        background: #56606d;
        border-radius: 4px;
    }

    :deep(.info__box__arrow) {
        width: 10px;
        height: 10px;
        background: #56606d;
        margin-bottom: -3px;
    }
</style>
