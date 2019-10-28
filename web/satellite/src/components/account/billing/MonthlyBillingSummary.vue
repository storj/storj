// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="current-month-area">
        <div class="current-month-area__header">
            <div class="current-month-area__header__month-info">
                <h1 class="current-month-area__header__month-info__title">Current Month</h1>
                <h2 class="current-month-area__header__month-info__title-info">{{currentPeriod}}</h2>
            </div>
            <div class="current-month-area__header__usage-info" v-if="false">
                <span class="current-month-area__header__usage-info__data">Usage <b class="current-month-area__header__usage-info__data__bold-text">$12.44</b></span>
                <VButton
                    label="Earn Credits"
                    width="153px"
                    height="48px"
                />
            </div>
        </div>
        <div class="current-month-area__content">
            <h2 class="current-month-area__content__title">DETAILED SUMMARY</h2>
            <div class="current-month-area__content__usage-charges" @click="toggleUsageChargesPopup">
                <div class="current-month-area__content__usage-charges__head">
                    <div class="current-month-area__content__usage-charges__head__name">
                        <svg class="current-month-area__content__usage-charges__head__name__image" v-if="!areUsageChargesShown" width="8" height="14" viewBox="0 0 8 14" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path fill-rule="evenodd" clip-rule="evenodd" d="M0.328889 13.6272C-0.10963 13.1302 -0.10963 12.3243 0.328889 11.8273L4.58792 7L0.328889 2.17268C-0.10963 1.67565 -0.10963 0.869804 0.328889 0.372774C0.767408 -0.124258 1.47839 -0.124258 1.91691 0.372774L7.76396 7L1.91691 13.6272C1.47839 14.1243 0.767409 14.1243 0.328889 13.6272Z" fill="#2683FF"/>
                        </svg>
                        <svg class="current-month-area__content__usage-charges__head__name__image" v-if="areUsageChargesShown" width="14" height="8" viewBox="0 0 14 8" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path fill-rule="evenodd" clip-rule="evenodd" d="M0.372773 0.338888C0.869804 -0.112963 1.67565 -0.112963 2.17268 0.338888L7 4.72741L11.8273 0.338888C12.3243 -0.112963 13.1302 -0.112963 13.6272 0.338888C14.1243 0.790739 14.1243 1.52333 13.6272 1.97519L7 8L0.372773 1.97519C-0.124258 1.52333 -0.124258 0.790739 0.372773 0.338888Z" fill="#2683FF"/>
                        </svg>
                        <span>Usage Charges</span>
                    </div>
                    <span>Estimated total $82.44</span>
                </div>
                <div class="current-month-area__content__usage-charges__content" v-if="areUsageChargesShown" @click.stop>
                    <div class="item">
                        <span>Project 1</span>
                        <span>$21.22</span>
                    </div>
                    <div class="item">
                        <span>Project 2</span>
                        <span>$12.88</span>
                    </div>
                </div>
            </div>
            <div class="current-month-area__content__referral-credits" @click="toggleReferralCreditsPopup">
                <div class="current-month-area__content__referral-credits__head">
                    <div class="current-month-area__content__referral-credits__head__name">
                        <svg class="current-month-area__content__referral-credits__head__name__image" v-if="!areReferralCreditsShown" width="8" height="14" viewBox="0 0 8 14" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path fill-rule="evenodd" clip-rule="evenodd" d="M0.328889 13.6272C-0.10963 13.1302 -0.10963 12.3243 0.328889 11.8273L4.58792 7L0.328889 2.17268C-0.10963 1.67565 -0.10963 0.869804 0.328889 0.372774C0.767408 -0.124258 1.47839 -0.124258 1.91691 0.372774L7.76396 7L1.91691 13.6272C1.47839 14.1243 0.767409 14.1243 0.328889 13.6272Z" fill="#2683FF"/>
                        </svg>
                        <svg class="current-month-area__content__referral-credits__head__name__image" v-if="areReferralCreditsShown" width="14" height="8" viewBox="0 0 14 8" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path fill-rule="evenodd" clip-rule="evenodd" d="M0.372773 0.338888C0.869804 -0.112963 1.67565 -0.112963 2.17268 0.338888L7 4.72741L11.8273 0.338888C12.3243 -0.112963 13.1302 -0.112963 13.6272 0.338888C14.1243 0.790739 14.1243 1.52333 13.6272 1.97519L7 8L0.372773 1.97519C-0.124258 1.52333 -0.124258 0.790739 0.372773 0.338888Z" fill="#2683FF"/>
                        </svg>
                        <span>Referral Credits</span>
                    </div>
                    <span>(+$20.00)</span>
                </div>
                <div class="current-month-area__content__referral-credits__content" v-if="areReferralCreditsShown" @click.stop>
                    <div class="item">
                        <span>Credit 1</span>
                        <span>$21.22</span>
                    </div>
                    <div class="item">
                        <span>Credit 2</span>
                        <span>$12.88</span>
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VButton from '@/components/common/VButton.vue';

@Component({
    components: {
        VButton,
    },
})
export default class MonthlyBillingSummary extends Vue {
    private areUsageChargesShown: boolean = false;
    private areReferralCreditsShown: boolean = false;

    public get currentPeriod(): string {
        const months: string[] = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'];
        const now: Date = new Date();
        const monthNumber = now.getMonth();
        const date = now.getDate();
        const year = now.getFullYear();

        if (date === 1) {
            return `${months[monthNumber]} 1 ${year}`;
        }

        return `${months[monthNumber]} 1 - ${date} ${year}`;
    }

    public toggleUsageChargesPopup(): void {
        this.areUsageChargesShown = !this.areUsageChargesShown;
    }

    public toggleReferralCreditsPopup(): void {
        this.areReferralCreditsShown = !this.areReferralCreditsShown;
    }
}
</script>

<style scoped lang="scss">
    h1,
    h2,
    p,
    span {
        margin: 0;
        color: #354049;
    }

    .current-month-area {
        margin-bottom: 32px;
        padding: 40px;
        background-color: #FFFFFF;
        border-radius: 8px;
        font-family: 'font_regular';

        &__header {
            display: flex;
            align-items: center;
            justify-content: space-between;

            &__month-info {

                &__title {
                    font-family: 'font_bold';
                    font-size: 32px;
                    line-height: 48px;
                }

                &__title-info {
                    font-size: 18px;
                }
            }

            &__usage-info {
                display: flex;
                align-items: center;

                &__data {
                    margin-right: 27px;
                    color: rgba(53, 64, 73, 0.5);
                    font-size: 18px;

                    &__bold-text {
                        color: #354049;
                    }
                }
            }
        }

        &__content {
            margin-top: 20px;

            &__title {
                font-size: 14px;
                line-height: 21px;
                color: #AFB7C1;
            }

            &__usage-charges {
                margin: 18px 0 0 0;
                padding: 20px 20px 20px 20px;
                background-color: #F5F6FA;
                border-radius: 12px;
                cursor: pointer;

                &__head {
                    display: flex;
                    justify-content: space-between;
                    align-items: center;

                    &__name {
                        display: flex;
                        align-items: center;

                        &__image {
                            margin-right: 12px;
                        }
                    }
                }

                &__content {
                    cursor: default;
                }
            }

            &__referral-credits {
                margin: 18px 0 12px 0;
                padding: 20px 20px 20px 20px;
                background-color: #F5F6FA;
                border-radius: 12px;
                cursor: pointer;

                &__head {
                    display: flex;
                    justify-content: space-between;
                    align-items: center;

                    &__name {
                        display: flex;
                        align-items: center;

                        &__image {
                            margin-right: 12px;
                        }
                    }
                }

                &__content {
                    cursor: default;
                }
            }
        }
    }

    .item {
        font-size: 16px;
        line-height: 21px;
        display: flex;
        justify-content: space-between;
        padding-top: 20px;
        margin-top: 20px;
        border-top: 1px solid rgba(169, 181, 193, 0.3);
    }
</style>
