// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="info" @mouseenter="toggleVisibility" @mouseleave="toggleVisibility">
        <slot class="slot"></slot>
        <div class="info__message-box" v-if="isVisible" :style="messageBoxStyle" :class="{extraPadding: isExtraPadding, customPosition: isCustomPosition}">
            <div class="info__message-box__text">
                <p class="info__message-box__text__regular-text">{{text}}</p>
                <p class="info__message-box__text__bold-text">{{boldText}}</p>
                <p class="info__message-box__text__bold-text">{{extraBoldText}}</p>
            </div>
            <div class="info__message-box__green-text" v-if="greenText">
                <p class="info__message-box__green-text__text">{{greenText}}</p>
                <p class="info__message-box__green-text__text">{{extraGreenText}}</p>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

declare interface MessageBoxStyle {
    bottom: string;
}

@Component
export default class VInfo extends Vue {
    private isVisible: boolean = false;
    private height: string = '5px';

    @Prop({default: ''})
    private readonly text: string;
    @Prop({default: ''})
    private readonly boldText: string;
    @Prop({default: ''})
    private readonly extraBoldText: string;
    @Prop({default: ''})
    private readonly greenText: string;
    @Prop({default: ''})
    private readonly extraGreenText: string;
    @Prop({default: false})
    private readonly isExtraPadding: boolean;
    @Prop({default: false})
    private readonly isCustomPosition: boolean;

    public toggleVisibility(): void {
        this.isVisible = !this.isVisible;
    }

    public get messageBoxStyle(): MessageBoxStyle {
        return { bottom: this.height };
    }

    public mounted(): void {
        const infoComponent = document.querySelector('.info');
        if (!infoComponent) {
            return;
        }

        const slots = this.$slots.default;
        if (!slots) {
            return;
        }

        const slot = slots[0];
        if (slot && slot.elm) {
            this.height = (slot.elm as HTMLElement).offsetHeight + 'px';
        }
    }
}
</script>

<style scoped lang="scss">
    p {
        margin-block-start: 0;
        margin-block-end: 0;
    }

    .info {
        position: relative;

        &__message-box {
            position: absolute;
            left: 50%;
            transform: translate(-50%);
            height: auto;
            width: auto;
            white-space: nowrap;
            display: flex;
            justify-content: space-between;
            align-items: center;
            text-align: center;
            background-image: var(--info-image-arrow-middle-path);
            background-size: 100% 100%;
            z-index: 101;
            padding: 11px 18px 20px 18px;

            &__text {
                display: flex;
                flex-direction: column;
                align-items: center;
                justify-content: center;

                &__bold-text {
                    color: var(--regular-text-color);
                    font-size: 12px;
                    line-height: 16px;
                    font-family: 'font_bold', sans-serif;
                }

                &__regular-text {
                    color: var(--regular-text-color);
                    font-size: 12px;
                    line-height: 16px;
                }
            }

            &__green-text {
                font-size: 12px;
                line-height: 16px;
                font-family: 'font_medium', sans-serif;
                color: #00ce7d;
                width: auto;
                margin-left: 10px;
            }
        }
    }

    .extraPadding {
        padding: 11px 18px 31px 18px;
    }

    .customPosition {
        left: 40%;
    }

    @media screen and (max-width: 500px) {

        .info__message-box {
            display: none;
        }
    }
</style>
