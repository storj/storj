// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="usage-charge-item-container">
        <div class="usage-charge-item-container__summary" @click="toggleDetailedInfo">
            <div class="usage-charge-item-container__summary__name-container">
                <svg class="usage-charge-item-container__summary__name-container__expand-image" v-if="!isDetailedInfoShown" width="8" height="14" viewBox="0 0 8 14" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path fill-rule="evenodd" clip-rule="evenodd" d="M0.328889 13.6272C-0.10963 13.1302 -0.10963 12.3243 0.328889 11.8273L4.58792 7L0.328889 2.17268C-0.10963 1.67565 -0.10963 0.869804 0.328889 0.372774C0.767408 -0.124258 1.47839 -0.124258 1.91691 0.372774L7.76396 7L1.91691 13.6272C1.47839 14.1243 0.767409 14.1243 0.328889 13.6272Z" fill="#2683FF"/>
                </svg>
                <svg class="usage-charge-item-container__summary__name-container__expand-image" v-if="isDetailedInfoShown" width="14" height="8" viewBox="0 0 14 8" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path fill-rule="evenodd" clip-rule="evenodd" d="M0.372773 0.338888C0.869804 -0.112963 1.67565 -0.112963 2.17268 0.338888L7 4.72741L11.8273 0.338888C12.3243 -0.112963 13.1302 -0.112963 13.6272 0.338888C14.1243 0.790739 14.1243 1.52333 13.6272 1.97519L7 8L0.372773 1.97519C-0.124258 1.52333 -0.124258 0.790739 0.372773 0.338888Z" fill="#2683FF"/>
                </svg>
                <span>Project 2</span>
            </div>
            <span class="small-font-size">$12.88</span>
        </div>
        <div class="usage-charge-item-container__detailed-info-container" v-if="isDetailedInfoShown">
            <div class="usage-charge-item-container__detailed-info-container__item">
                <span>Storage</span>
                <span>$18.00</span>
            </div>
            <div class="usage-charge-item-container__detailed-info-container__item">
                <span>Egress</span>
                <span>$2.00</span>
            </div>
            <div class="usage-charge-item-container__detailed-info-container__item">
                <span>Objects</span>
                <span>$1.22</span>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

@Component
export default class UsageChargeItem extends Vue {
    public isDetailedInfoShown: boolean = false;

    public toggleDetailedInfo(): void {
        this.isDetailedInfoShown = !this.isDetailedInfoShown;
    }
}
</script>

<style scoped lang="scss">
    .usage-charge-item-container {
        font-size: 16px;
        line-height: 21px;
        padding: 20px 0;

        &__summary {
            display: flex;
            justify-content: space-between;
            align-items: center;

            &__name-container {
                display: flex;
                align-items: center;

                &__expand-image {
                    width: 14px;
                    height: 14px;
                    margin-right: 12px;
                }
            }
        }

        &__detailed-info-container {
            display: flex;
            flex-direction: column;
            align-items: flex-start;
            justify-content: space-between;
            padding: 16px 0 0 26px;

            &__item {
                display: flex;
                justify-content: space-between;
                align-items: center;
                width: 100%;
                font-family: 'font_regular', sans-serif;
                font-size: 14px;
                line-height: 18px;
                color: #6f7a83;
                margin-top: 5px;
            }
        }
    }

    .small-font-size {
        font-size: 14px;
        line-height: 18px;
    }
</style>
