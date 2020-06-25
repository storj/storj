// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="held-history-table-container--small__item">
        <div class="held-history-table-container--small__item__satellite-info">
            <div>
                <p class="held-history-table-container--small__item__satellite-info__name">{{ heldHistoryItem.satelliteName }}</p>
                <p class="held-history-table-container--small__item__satellite-info__months">{{ heldHistoryItem.age }} month</p>
            </div>
            <div class="held-history-table-container--small__item__satellite-info__button">
                <div class="icon hide" @click="hide" v-if="isExpanded">
                    <blue-hide-icon></blue-hide-icon>
                </div>
                <div class="icon expand" @click="expand" v-else>
                    <blue-expand-icon></blue-expand-icon>
                </div>
            </div>
        </div>
        <transition name="fade">
            <div class="held-history-table-container--small__item__held-info" v-if="isExpanded">
                <div class="held-history-table-container--small__item__held-info__item">
                    <p class="held-history-table-container--small__item__held-info__item__label">Month 1-3</p>
                    <p class="held-history-table-container--small__item__held-info__item__value">{{ heldHistoryItem.firstPeriod | centsToDollars }}</p>
                </div>
                <div class="held-history-table-container--small__item__held-info__item">
                    <p class="held-history-table-container--small__item__held-info__item__label">Month 4-6</p>
                    <p class="held-history-table-container--small__item__held-info__item__value">{{ heldHistoryItem.secondPeriod | centsToDollars }}</p>
                </div>
                <div class="held-history-table-container--small__item__held-info__item">
                    <p class="held-history-table-container--small__item__held-info__item__label">Month 7-9</p>
                    <p class="held-history-table-container--small__item__held-info__item__value">{{ heldHistoryItem.thirdPeriod | centsToDollars }}</p>
                </div>
            </div>
        </transition>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import BlueHideIcon from '@/../static/images/common/BlueMinus.svg';
import BlueExpandIcon from '@/../static/images/common/BluePlus.svg';

import { HeldHistoryMonthlyBreakdownItem } from '@/app/types/payout';

@Component({
    components: {
        BlueExpandIcon,
        BlueHideIcon,
    },
})
export default class HeldHistoryMonthlyBreakdownTableSmall extends Vue {
    @Prop({default: () => new HeldHistoryMonthlyBreakdownItem()})
    public readonly heldHistoryItem: HeldHistoryMonthlyBreakdownItem;

    /**
     * Indicates if held info should be rendered.
     */
    public isExpanded: boolean = false;

    /**
     * Shows held info.
     */
    public expand(): void {
        this.isExpanded = true;
    }

    /**
     * Hides held info.
     */
    public hide(): void {
        this.isExpanded = false;
    }
}
</script>

<style scoped lang="scss">
    .held-history-table-container--small__item {
        padding: 12px;
        width: calc(100% - 24px);

        &__satellite-info {
            display: flex;
            align-items: center;
            justify-content: space-between;

            &__name {
                font-family: 'font_regular', sans-serif;
                font-size: 14px;
                color: var(--regular-text-color);
                max-width: calc(100% - 40px);
                word-break: break-word;
            }

            &__months {
                font-family: 'font_regular', sans-serif;
                font-size: 11px;
                color: #9b9db1;
                margin-top: 3px;
            }

            &__button {
                display: flex;
                align-items: center;
                justify-content: center;
                width: 30px;
                height: 30px;
                min-width: 30px;
                min-height: 30px;
                background: var(--expand-button-background-color);
                border-radius: 3px;
                cursor: pointer;
            }
        }

        &__held-info {
            margin-top: 16px;

            &__item {
                display: flex;
                align-items: center;
                justify-content: space-between;
                font-size: 12px;
                line-height: 12px;
                margin-bottom: 10px;

                &__label {
                    font-family: 'font_medium', sans-serif;
                    color: #909bad;
                }

                &__value {
                    font-family: 'font_regular', sans-serif;
                    color: var(--regular-text-color);
                }
            }
        }
    }

    .icon {
        display: flex;
        align-items: center;
        justify-content: center;
        max-width: 100%;
        max-height: 100%;
        width: 100%;
        height: 100%;
    }

    .fade-enter-active,
    .fade-leave-active {
        transition: opacity 0.5s;
    }

    .fade-enter,
    .fade-leave-to /* .fade-leave-active below version 2.1.8 */ {
        opacity: 0;
    }
</style>
