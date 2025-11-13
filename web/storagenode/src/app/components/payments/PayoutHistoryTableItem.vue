// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="payout-history-item">
        <div class="payout-history-item__header" @click="toggleExpanded">
            <div class="payout-history-item__header__left-area">
                <div class="payout-history-item__header__left-area__expand-icon">
                    <ExpandIcon />
                </div>
                <p>{{ historyItem.satelliteName }}</p>
            </div>
            <p class="payout-history-item__header__total">{{ centsToDollars(historyItem.paid) }}</p>
        </div>
        <transition name="fade" mode="in-out">
            <div v-if="isExpanded" class="payout-history-item__expanded-area">
                <div class="payout-history-item__expanded-area__left-area">
                    <div class="payout-history-item__expanded-area__left-area__info-area">
                        <div class="payout-history-item__expanded-area__left-area__info-area__item flex-start">
                            <p class="payout-history-item__expanded-area__left-area__info-area__item__label extra-margin">Node Age</p>
                            <p class="payout-history-item__expanded-area__left-area__info-area__item__value">{{ `${historyItem.age} Month${historyItem.age > 1 ? 's' : ''}` }}</p>
                        </div>
                        <div class="payout-history-item__expanded-area__left-area__info-area__item flex-start">
                            <p class="payout-history-item__expanded-area__left-area__info-area__item__label extra-margin">Earned</p>
                            <p class="payout-history-item__expanded-area__left-area__info-area__item__value">{{ centsToDollars(historyItem.earned) }}</p>
                        </div>
                        <div class="payout-history-item__expanded-area__left-area__info-area__item flex-end">
                            <div class="row extra-margin">
                                <p class="payout-history-item__expanded-area__left-area__info-area__item__label">Surge</p>
                                <div class="payout-history-item__expanded-area__left-area__info-area__item__info-block">
                                    <p class="payout-history-item__expanded-area__left-area__info-area__item__info-block__text">{{ historyItem.surgePercent + '%' }}</p>
                                </div>
                            </div>
                            <p class="payout-history-item__expanded-area__left-area__info-area__item__value">{{ centsToDollars(historyItem.surge) }}</p>
                        </div>
                        <div class="payout-history-item__expanded-area__left-area__info-area__item flex-end">
                            <div class="row extra-margin">
                                <p class="payout-history-item__expanded-area__left-area__info-area__item__label">Held</p>
                                <div class="payout-history-item__expanded-area__left-area__info-area__item__info-block">
                                    <p class="payout-history-item__expanded-area__left-area__info-area__item__info-block__text">{{ historyItem.heldPercent + '%' }}</p>
                                </div>
                            </div>
                            <p class="payout-history-item__expanded-area__left-area__info-area__item__value">{{ centsToDollars(historyItem.held) }}</p>
                        </div>
                    </div>
                    <div class="payout-history-item__expanded-area__left-area__footer">
                        <div v-if="historyItem.isExitComplete" class="payout-history-item__expanded-area__left-area__footer__item">
                            <DisqualifyIcon class="disqualify-icon" />
                            <div class="payout-history-item__expanded-area__left-area__footer__item__text-area">
                                <p class="payout-history-item__expanded-area__left-area__footer__item__text-area__title">
                                    Your node made Graceful Exit
                                </p>
                                <p class="payout-history-item__expanded-area__left-area__footer__item__text-area__text">
                                    100% of total withholdings are returned
                                </p>
                            </div>
                        </div>
                        <div v-if="historyItem.age > 9 && !historyItem.isExitComplete" class="payout-history-item__expanded-area__left-area__footer__item">
                            <OKIcon class="ok-icon" />
                            <div class="payout-history-item__expanded-area__left-area__footer__item__text-area">
                                <p class="payout-history-item__expanded-area__left-area__footer__item__text-area__title">
                                    Your node reached age of 10 Month
                                </p>
                                <p class="payout-history-item__expanded-area__left-area__footer__item__text-area__text">
                                    100% of storage node revenue is paid
                                </p>
                            </div>
                        </div>
                    </div>
                </div>
                <div class="payout-history-item__expanded-area__right-area">
                    <p class="payout-history-item__expanded-area__right-area__label flex-end">Paid</p>
                    <div class="payout-history-item__expanded-area__right-area__info-item">
                        <p class="payout-history-item__expanded-area__right-area__info-item__label">After Held</p>
                        <p class="payout-history-item__expanded-area__right-area__info-item__value">{{ centsToDollars(historyItem.afterHeld) }}</p>
                    </div>
                    <div class="payout-history-item__expanded-area__right-area__info-item">
                        <p class="payout-history-item__expanded-area__right-area__info-item__label">Held Returned</p>
                        <p class="payout-history-item__expanded-area__right-area__info-item__value">{{ centsToDollars(historyItem.disposed) }}</p>
                    </div>
                    <div class="payout-history-item__expanded-area__right-area__info-item">
                        <p class="payout-history-item__expanded-area__right-area__info-item__label">Distributed</p>
                        <p class="payout-history-item__expanded-area__right-area__info-item__value">{{ centsToDollars(historyItem.distributed) }}</p>
                    </div>
                    <div class="payout-history-item__expanded-area__right-area__divider" />
                    <div class="payout-history-item__expanded-area__right-area__footer">
                        <div v-if="historyItem.transactionLink" class="payout-history-item__expanded-area__right-area__footer__transaction">
                            <a
                                class="payout-history-item__expanded-area__right-area__footer__transaction__link"
                                :href="historyItem.transactionLink"
                                target="_blank"
                                rel="noreferrer noopener"
                            >
                                Transaction
                            </a>
                            <ShareIcon class="payout-history-item__expanded-area__right-area__footer__transaction__icon" />
                        </div>
                        <p class="payout-history-item__expanded-area__right-area__footer__total">{{ centsToDollars(historyItem.distributed > 0 ? historyItem.distributed : historyItem.paid) }}</p>
                    </div>
                </div>
            </div>
        </transition>
    </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';

import { SatellitePayoutForPeriod } from '@/storagenode/payouts/payouts';
import { centsToDollars } from '@/app/utils/payout';

import ExpandIcon from '@/../static/images/BlueArrowRight.svg';
import DisqualifyIcon from '@/../static/images/largeDisqualify.svg';
import OKIcon from '@/../static/images/payments/OKIcon.svg';
import ShareIcon from '@/../static/images/payments/Share.svg';

withDefaults(defineProps<{
    historyItem?: SatellitePayoutForPeriod;
}>(), {
    historyItem: () => new SatellitePayoutForPeriod(),
});

const isExpanded = ref<boolean>(false);

function toggleExpanded(): void {
    isExpanded.value = !isExpanded.value;
}
</script>

<style scoped lang="scss">
    .payout-history-item {
        padding: 17px;
        width: calc(100% - 34px);
        border-bottom: 1px solid #eaeaea;

        &__header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            font-family: 'font_medium', sans-serif;
            font-size: 14px;
            color: var(--regular-text-color);
            cursor: pointer;

            &__left-area {
                display: flex;
                align-items: center;
                justify-content: flex-start;

                &__expand-icon {
                    display: flex;
                    align-items: center;
                    justify-content: center;
                    width: 14px;
                    height: 14px;
                    max-width: 14px;
                    max-height: 14px;
                    margin-right: 12px;
                }
            }
        }

        &__expanded-area {
            position: relative;
            display: flex;
            align-items: flex-start;
            justify-content: space-between;
            padding-left: 26px;
            width: calc(100% - 26px);
            margin-top: 20px;

            &__left-area {
                display: flex;
                flex-direction: column;
                justify-content: flex-start;
                align-items: flex-start;
                width: 65%;

                &__info-area {
                    display: flex;
                    align-items: center;
                    justify-content: space-between;
                    width: 100%;

                    &__item {
                        display: flex;
                        flex-direction: column;
                        font-family: 'font_medium', sans-serif;
                        font-size: 14px;

                        &__label {
                            color: var(--label-text-color);
                        }

                        &__value {
                            color: var(--regular-text-color);
                        }

                        &__info-block {
                            margin-left: 5px;
                            border-radius: 4px;
                            background: #909bad;
                            padding: 3px;

                            &__text {
                                font-size: 11px;
                                line-height: 11px;
                                color: white;
                            }
                        }
                    }
                }

                &__footer {
                    margin-top: 30px;
                    width: 100%;

                    &__item {
                        display: flex;
                        justify-content: flex-start;
                        align-items: center;

                        &__text-area {
                            display: flex;
                            flex-direction: column;
                            align-items: flex-start;
                            justify-content: center;
                            margin-left: 12px;

                            &__title {
                                font-family: 'font_bold', sans-serif;
                                font-size: 15px;
                                color: var(--regular-text-color);
                            }

                            &__text {
                                margin-top: 6px;
                                font-family: 'font_regular', sans-serif;
                                font-size: 13px;
                                color: var(--regular-text-color);
                            }
                        }
                    }
                }
            }

            &__right-area {
                width: 35%;
                display: flex;
                flex-direction: column;
                font-family: 'font_medium', sans-serif;
                font-size: 14px;

                &__label {
                    color: var(--label-text-color);
                    text-align: end;
                }

                &__info-item {
                    display: flex;
                    align-items: center;
                    justify-content: flex-end;

                    &__label,
                    &__value {
                        margin-top: 10px;
                        color: var(--regular-text-color);
                    }

                    &__value {
                        margin-left: 30px;
                        width: 60px;
                        text-align: end;
                    }
                }

                &__divider {
                    width: 100%;
                    height: 1px;
                    background: #eaeaea;
                    margin: 15px 0;
                }

                &__footer {
                    display: flex;
                    align-items: flex-end;
                    justify-content: flex-end;

                    &__transaction {
                        display: flex;
                        align-items: center;
                        justify-content: flex-end;
                        color: var(--navigation-link-color);
                        cursor: pointer;

                        a:visited {
                            color: var(--navigation-link-color);
                        }

                        &__icon {
                            margin-left: 7px;

                            :deep(path) {
                                stroke: var(--navigation-link-color);
                            }
                        }
                    }

                    &__total {
                        margin-left: 30px;
                        width: 60px;
                        font-family: 'font_bold', sans-serif;
                        color: var(--regular-text-color);
                        text-align: end;
                    }
                }
            }
        }
    }

    .row {
        display: flex;
        align-items: flex-end;
        justify-content: center;
    }

    .extra-margin {
        margin-bottom: 8px;
    }

    .flex-start {
        justify-content: flex-start;
        align-items: flex-start;
    }

    .flex-end {
        justify-content: flex-end;
        align-items: flex-end;
    }

    .fade-enter-active,
    .fade-leave-active {
        transition: opacity 0.5s;
    }

    .fade-enter,
    .fade-leave-to {
        opacity: 0;
    }

    .disqualify-icon {
        width: 40px;
        height: 40px;
        min-width: 40px;
        min-height: 40px;

        :deep(path) {
            fill: #909bad;
        }
    }

    .ok-icon {
        width: 30px;
        height: 30px;
        min-width: 30px;
        min-height: 30px;
    }

    @media screen and (width <= 800px) {

        .payout-history-item {
            padding: 17px 10px 12px;
            width: calc(100% - 20px);

            &__header {
                flex-wrap: wrap;

                &__left-area {
                    flex-wrap: nowrap;
                    margin-bottom: 5px;
                }

                &__total {
                    margin-bottom: 5px;
                }
            }

            &__expanded-area {
                width: 100%;
                padding: 0;
                flex-direction: column;

                &__left-area {
                    width: 100%;

                    &__info-area {
                        flex-wrap: wrap;
                        width: 100%;

                        &__item {
                            align-items: flex-start;
                            justify-content: flex-start;
                            margin-bottom: 5px;
                        }
                    }

                    &__footer {
                        width: 100%;
                    }
                }

                &__right-area {
                    width: 100%;
                }
            }
        }
    }
</style>
