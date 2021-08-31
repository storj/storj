// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="info" @mouseenter="toggleVisibility" @mouseleave="toggleVisibility">
        <slot />
        <div v-if="isVisible" class="info__box">
            <div v-if="buttonLabel" class="info__box__click-mock" />
            <div class="info__box__arrow" />
            <div class="info__box__message">
                <h1 v-if="title" class="info__box__message__title">{{ title }}</h1>
                <p class="info__box__message__regular-text">{{ text }}</p>
                <p class="info__box__message__bold-text">{{ boldText }}</p>
                <VButton
                    v-if="buttonLabel"
                    class="info__box__message__button"
                    :label="buttonLabel"
                    height="42px"
                    border-radius="52px"
                    :on-press="onClick"
                />
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import VButton from '@/components/common/VButton.vue';

// @vue/component
@Component({
    components: {
        VButton,
    }
})
export default class VInfo extends Vue {
    @Prop({default: ''})
    private readonly title: string;
    @Prop({default: ''})
    private readonly text: string;
    @Prop({default: ''})
    private readonly boldText: string;
    @Prop({default: ''})
    private readonly buttonLabel: string;
    @Prop({default: () => false})
    private readonly onButtonClick: () => unknown;

    public isVisible = false;

    /**
     * Holds on button click logic.
     */
    public onClick(): void {
        this.onButtonClick();
        this.toggleVisibility();
    }

    /**
     * Toggles bubble visibility.
     */
    public toggleVisibility(): void {
        this.isVisible = !this.isVisible;
    }
}
</script>

<style scoped lang="scss">
    .info {
        position: relative;

        &__box {
            position: absolute;
            top: calc(100% + 10px);
            left: calc(50% + 1px);
            transform: translate(-50%);
            display: flex;
            flex-direction: column;
            align-items: center;
            filter: drop-shadow(0 0 34px #0a1b2c47);
            z-index: 1;

            &__click-mock {
                height: 24px;
                background: transparent;
            }

            &__arrow {
                background-color: white;
                width: 40px;
                height: 40px;
                border-radius: 4px 0 0 0;
                transform: scale(1, 0.85) translate(0, 20%) rotate(45deg);
                margin-bottom: -15px;
            }

            &__message {
                box-sizing: border-box;
                background-color: white;
                padding: 24px;
                border-radius: 20px;

                &__title {
                    font-family: 'font_bold', sans-serif;
                    font-size: 14px;
                    line-height: 32px;
                    color: #000;
                    margin-bottom: 10px;
                }

                &__bold-text,
                &__regular-text {
                    color: #586c86;
                    font-family: 'font_medium', sans-serif;
                    margin: 0;
                    font-size: 16px;
                    line-height: 18px;
                }

                &__regular-text {
                    font-family: 'font_regular', sans-serif;
                }

                &__button {
                    margin-top: 20px;
                }
            }
        }
    }
</style>
