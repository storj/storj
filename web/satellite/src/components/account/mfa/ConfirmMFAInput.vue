// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="confirm-mfa">
        <label for="confirm-mfa" class="confirm-mfa__label">
            <span class="confirm-mfa__label__info">2FA Code</span>
            <span class="confirm-mfa__label__error" v-if="isError">Invalid code. Please re-enter.</span>
        </label>
        <input
            id="confirm-mfa"
            class="confirm-mfa__input"
            placeholder="000000"
            type="number"
            @input="event => onInput(event.target.value)"
        >
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

@Component
export default class ConfirmMFAInput extends Vue {
    @Prop({default: () => false})
    public readonly onInput: (value: string) => void;
    @Prop({default: false})
    public readonly isError: boolean;
}
</script>

<style scoped lang="scss">
    .confirm-mfa {
        width: 100%;

        &__label {
            display: flex;
            align-items: center;
            justify-content: space-between;

            &__info {
                font-size: 16px;
                line-height: 21px;
                color: #354049;
            }

            &__error {
                font-family: 'font_medium', sans-serif;
                font-size: 16px;
                line-height: 21px;
                text-align: right;
                color: #ce3030;
            }
        }

        &__input {
            width: calc(100% - 40px);
            margin-top: 5px;
            background: #fff;
            border: 1px solid #a9b5c1;
            border-radius: 6px;
            padding: 15px 20px;
            font-size: 16px;

            /* Chrome, Safari, Edge, Opera */

            &::-webkit-outer-spin-button,
            &::-webkit-inner-spin-button {
                -webkit-appearance: none;
                margin: 0;
            }
        }
    }

    /* Firefox */

    input[type=number] {
        -moz-appearance: textfield;
    }
</style>
