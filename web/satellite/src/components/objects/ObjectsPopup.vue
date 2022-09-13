// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="objects-popup">
        <div class="objects-popup__container">
            <h1 class="objects-popup__container__title">{{ title }}</h1>
            <p class="objects-popup__container__sub-title">{{ subTitle }}</p>
            <div class="objects-popup__container__info">
                <WarningIcon />
                <p class="objects-popup__container__info__msg">Only lowercase alphanumeric characters are allowed.</p>
            </div>
            <VInput
                class="objects-popup__container__input"
                label="Bucket Name"
                placeholder="Enter bucket name"
                :error="errorMessage"
                :is-loading="isLoading"
                @setData="onChangeName"
            />
            <VButton
                :label="buttonLabel"
                width="100%"
                height="48px"
                :on-press="onClick"
                :is-disabled="isLoading"
            />
            <div class="objects-popup__container__close-cross-container" @click="onCloseClick">
                <CloseCrossIcon />
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import VInput from '@/components/common/VInput.vue';
import VButton from '@/components/common/VButton.vue';

import CloseCrossIcon from '@/../static/images/common/closeCross.svg';
import WarningIcon from '@/../static/images/objects/warning.svg';

// @vue/component
@Component({
    components: {
        VInput,
        VButton,
        CloseCrossIcon,
        WarningIcon,
    },
})
export default class ObjectsPopup extends Vue {
    @Prop({ default: () => () => {} })
    public readonly onClick: () => void;
    @Prop({ default: '' })
    public readonly title: string;
    @Prop({ default: '' })
    public readonly subTitle: string;
    @Prop({ default: '' })
    public readonly buttonLabel: string;
    @Prop({ default: '' })
    public readonly errorMessage: string;
    @Prop({ default: false })
    public readonly isLoading: boolean;

    /**
     * Sets bucket name from input.
     */
    public onChangeName(value: string): void {
        this.$emit('setName', value);
    }

    /**
     * Closes popup.
     */
    public onCloseClick(): void {
        this.$emit('close');
    }
}
</script>

<style scoped lang="scss">
    .objects-popup {
        position: fixed;
        top: 0;
        bottom: 0;
        left: 0;
        right: 0;
        background: rgb(27 37 51 / 75%);
        display: flex;
        align-items: center;
        justify-content: center;

        &__container {
            padding: 45px 70px;
            border-radius: 10px;
            font-family: 'font_regular', sans-serif;
            font-style: normal;
            display: flex;
            flex-direction: column;
            align-items: center;
            background-color: #fff;
            max-width: 480px;
            position: relative;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 22px;
                line-height: 27px;
                color: #000;
                margin: 0 0 18px;
            }

            &__sub-title {
                font-weight: normal;
                font-size: 18px;
                line-height: 30px;
                text-align: center;
                letter-spacing: -0.1007px;
                color: rgb(37 37 37 / 70%);
                margin: 0;
            }

            &__info {
                display: flex;
                align-items: center;
                padding: 23px 14px;
                background: #f5f6fa;
                border: 1px solid #a9b5c1;
                margin: 20px 0;
                border-radius: 9px;

                &__msg {
                    font-family: 'font_medium', sans-serif;
                    font-size: 16px;
                    line-height: 19px;
                    color: #1b2533;
                    margin: 0 0 0 10px;
                }
            }

            &__input {
                margin-bottom: 18px;
            }

            &__close-cross-container {
                display: flex;
                justify-content: center;
                align-items: center;
                position: absolute;
                right: 30px;
                top: 30px;
                height: 24px;
                width: 24px;
                cursor: pointer;

                &:hover .close-cross-svg-path {
                    fill: #2683ff;
                }
            }
        }
    }
</style>
