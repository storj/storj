// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <info-block>
        <div slot="body" class="details-area">
            <header class="details-area__header">
                <h3 class="details-area__header__title">Details</h3>
                <p class="details-area__header__period">{{ period }}</p>
            </header>
            <div class="details-area__content">
                <div class="details-area__content__item">
                    <p class="details-area__content__item__label">EARNED</p>
                    <p class="details-area__content__item__value">{{ totalEarned | centsToDollars }}</p>
                </div>
                <div class="details-area__content__item">
                    <p class="details-area__content__item__label">HELD</p>
                    <p class="details-area__content__item__value">{{ totalHeld | centsToDollars }}</p>
                </div>
                <div class="details-area__content__item">
                    <p class="details-area__content__item__label">PAID</p>
                    <p class="details-area__content__item__value">{{ totalPaid | centsToDollars }}</p>
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
export default class DetailsArea extends Vue {
    @Prop({ default: 0 })
    public totalEarned: number;
    @Prop({ default: 0 })
    public totalHeld: number;
    @Prop({ default: 0 })
    public totalPaid: number;
    @Prop({ default: '' })
    public period: string;
}
</script>

<style scoped lang="scss">
    .details-area {
        box-sizing: border-box;
        width: 100%;

        &__header {
            display: flex;
            align-items: flex-end;
            justify-content: space-between;
            margin-bottom: 30px;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 24px;
                line-height: 24px;
                color: var(--v-header-base);
            }

            &__period {
                font-family: 'font_regular', sans-serif;
                font-size: 16px;
                color: var(--v-text-base);
            }
        }

        &__content {
            display: flex;
            align-items: flex-end;
            justify-content: space-between;

            &__item {
                display: flex;
                flex-direction: column;
                align-items: flex-start;
                font-family: 'font_semiBold', sans-serif;

                &:last-of-type {
                    align-items: flex-end;
                }

                &__label {
                    font-size: 12px;
                    color: var(--v-text-base);
                    margin-bottom: 10px;
                }

                &__value {
                    font-size: 18px;
                    color: var(--v-header-base);
                }
            }
        }
    }
</style>
