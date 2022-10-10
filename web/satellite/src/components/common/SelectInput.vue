// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="input-wrap">
        <div class="label-container">
            <p v-if="label" class="label-container__label" :style="style.labelStyle">{{ label }}</p>
        </div>
        <InputCaret v-if="optionsList.length > 0" class="select-input__caret" />
        <select
            v-model="value"
            :style="style.inputStyle"
            class="select-input"
            @change="onInput"
        >
            <option
                v-for="(option, index) in optionsList"
                :key="index"
                class="select-input__option"
                :value="option"
            >
                {{ option }}
            </option>
        </select>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import InputCaret from '@/../static/images/common/caret.svg';

// Custom input component for login page
// @vue/component
@Component({
    components: {
        InputCaret,
    },
})
export default class SelectInput extends Vue {

    protected value = '';

    @Prop({ default: '' })
    protected readonly label: string;
    @Prop({ default: '48px' })
    protected readonly height: string;
    @Prop({ default: '100%' })
    protected readonly width: string;
    @Prop({ default: () => [] })
    protected readonly optionsList: string[];

    @Prop({ default: false })
    private readonly isWhite: boolean;

    public created() {
        this.value = this.optionsList ? this.optionsList[0] : '';
        this.$emit('setData', this.value);
    }

    /**
     * triggers on input.
     */
    public onInput(event: Event): void {
        const target = event.target as HTMLSelectElement;
        this.$emit('setData', target.value);
    }

    /**
     * Returns style objects depends on props.
     */
    protected get style(): Record<string, unknown> {
        return {
            inputStyle: {
                width: this.width,
                height: this.height,
            },
            labelStyle: {
                color: this.isWhite ? 'white' : '#354049',
            },
        };
    }
}
</script>

<style scoped lang="scss">
    .input-wrap {
        position: relative;
        width: 100%;
        font-family: 'font_regular', sans-serif;

        .select-input {
            font-size: 16px;
            line-height: 21px;
            resize: none;
            height: 46px;
            padding: 0 30px 0 0;
            text-indent: 20px;
            border: 1px solid rgb(56 75 101 / 40%);
            border-radius: 6px;
            box-sizing: border-box;
            appearance: none;

            &__caret {
                position: absolute;
                right: 28px;
                bottom: 18px;
            }
        }
    }

    .label-container {
        display: flex;
        justify-content: flex-start;
        align-items: flex-end;
        padding-bottom: 8px;
        flex-direction: row;

        &__label {
            font-size: 16px;
            line-height: 21px;
            color: #354049;
            margin-bottom: 0;
        }
    }

</style>
