// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="checks-area-container">
        <div class="checks-area-container__header">
            <p class="checks-area-container__header__title">{{ label }}</p>
            <div class="checks-area-container__header__info-area">
                <ChecksInfoIcon class="checks-area-image" alt="Blue info icon with question mark" @mouseenter="toggleTooltipVisibility" @mouseleave="toggleTooltipVisibility" />
                <div v-show="isTooltipVisible" class="tooltip">
                    <div class="tooltip__text-area">
                        <p class="tooltip__text-area__text">{{ infoText }}</p>
                    </div>
                    <div class="tooltip__footer" />
                </div>
            </div>
        </div>
        <p class="checks-area-container__amount"><b>{{ amount }}</b></p>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import ChecksInfoIcon from '@/../static/images/checksInfo.svg';

// @vue/component
@Component ({
    components: {
        ChecksInfoIcon,
    },
})
export default class ChecksArea extends Vue {
    @Prop({default: ''})
    private readonly label: string;
    @Prop({default: ''})
    private readonly amount: string;
    @Prop({default: ''})
    private readonly infoText: string;

    /**
     * Indicates if tooltip needs to be shown.
     */
    public isTooltipVisible = false;

    /**
     * Toggles tooltip visibility.
     */
    public toggleTooltipVisibility(): void {
        this.isTooltipVisible = !this.isTooltipVisible;
    }
}
</script>

<style scoped lang="scss">
    .checks-area-container {
        width: calc(48% - 60px);
        height: 79px;
        background-color: var(--block-background-color);
        border: 1px solid var(--block-border-color);
        border-radius: 11px;
        padding: 32px 30px;
        margin-bottom: 13px;
        position: relative;

        &__header {
            display: flex;
            align-items: center;

            &__title {
                font-size: 14px;
                line-height: 21px;
                color: var(--title-text-color);
                margin: 0 5px 0 0;
            }

            .checks-area-image {
                margin-top: 3px;
                cursor: pointer;

                &:hover {

                    .checks-area-svg-rect {
                        fill: #a5c7ef;
                    }
                }
            }

            &__info-area {
                position: relative;
            }
        }

        &__amount {
            font-size: 32px;
            line-height: 57px;
            color: var(--regular-text-color);
            margin: 0;
        }
    }

    .tooltip {
        position: absolute;
        bottom: 35px;
        left: 50%;
        transform: translate(-50%);
        height: auto;
        box-shadow: 0 2px 48px var(--tooltip-shadow-color);
        border-radius: 12px;
        background: var(--tooltip-background-color);

        &__text-area {
            padding: 15px 11px;
            width: 178px;
            font-family: 'font_regular', sans-serif;
            font-size: 11px;
            line-height: 17px;
            color: var(--regular-text-color);
            text-align: center;
        }

        &__footer {
            position: absolute;
            left: 50%;
            transform: translate(-50%);
            width: 0;
            height: 0;
            border-style: solid;
            border-width: 11.5px 11.5px 0;
            border-color: var(--tooltip-background-color) transparent transparent transparent;
        }
    }

    @media screen and (max-width: 460px) {

        .checks-area-image {
            display: none;
        }
    }
</style>
