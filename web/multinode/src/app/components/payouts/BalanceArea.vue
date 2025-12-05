// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <info-block>
        <div slot="body" class="balance-area">
            <header class="balance-area__header">
                <h3 class="balance-area__header__title">Balance</h3>
            </header>
            <div class="balance-area__content">
                <div class="balance-area__content__item">
                    <p class="balance-area__content__item__label">Undistributed Payouts</p>
                    <p class="balance-area__content__item__value">{{ undistributed | centsToDollars }}</p>
                </div>
                <div class="balance-area__content__item">
                    <p class="balance-area__content__item__label">Current Month Estimated Earnings</p>
                    <p class="balance-area__content__item__value">{{ currentMonthEstimation | centsToDollars }}</p>
                </div>
            </div>
        </div>
    </info-block>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import InfoBlock from '../common/InfoBlock.vue';

// @vue/component
@Component({
    components: { InfoBlock },
})
export default class BalanceArea extends Vue {
    @Prop({ default: 0 })
    public undistributed: number;
    @Prop({ default: 0 })
    public currentMonthEstimation: number;
}
</script>

<style scoped lang="scss">
    .balance-area {
        box-sizing: border-box;
        width: 100%;

        &__header {

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 24px;
                line-height: 24px;
                color: var(--c-header-base);
            }
        }

        &__content {
            display: flex;
            flex-direction: column;
            align-items: flex-start;
            margin-top: 34px;

            &__item {
                display: flex;
                align-items: flex-end;
                justify-content: space-between;
                width: 100%;

                &:not(:first-of-type) {
                    margin-top: 26px;
                }

                &__label {
                    font-size: 14px;
                    color: var(--v-text-base);
                    font-family: 'font_medium', sans-serif;
                }

                &__value {
                    font-size: 18px;
                    color: var(--v-header-base);
                    font-family: 'font_semiBold', sans-serif;
                }
            }
        }
    }
</style>
