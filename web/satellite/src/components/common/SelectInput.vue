// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="input-wrap">
        <div class="label-container">
            <p v-if="label" class="label-container__label" :style="style.labelStyle">{{ label }}</p>
        </div>
        <input-caret v-if="optionsList.length > 0" class="select-input__caret" />
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

<script setup lang="ts">
import { computed, onBeforeMount, ref } from 'vue';

import InputCaret from '@/../static/images/common/caret.svg';

const props = withDefaults(defineProps<{
    label?: string;
    height?: string;
    width?: string;
    optionsList?: string[];
    isWhite?: boolean;
}>(), {
    label: '',
    height: '48px',
    width: '100%',
    optionsList: () => [],
    isWhite: false,
});

const emit = defineEmits(['setData']);

const value = ref<string>('');

/**
 * Returns style objects depends on props.
 */
const style = computed((): Record<string, unknown> => {
    return {
        inputStyle: {
            width: props.width,
            height: props.height,
        },
        labelStyle: {
            color: props.isWhite ? 'white' : '#354049',
        },
    };
});

/**
 * triggers on input.
 */
function onInput(event: Event): void {
    const target = event.target as HTMLSelectElement;
    emit('setData', target.value);
}

onBeforeMount(() => {
    value.value = props.optionsList ? props.optionsList[0] : '';
    emit('setData', value.value);
});
</script>

<style scoped lang="scss">
    .input-wrap {
        position: relative;
        width: 100%;
        font-family: 'font_regular', sans-serif;

        .select-input {
            font-size: 15px;
            resize: none;
            height: 48px;
            padding: 0 30px 0 0;
            text-indent: 18px;
            border: 1px solid #ccc;
            border-radius: 6px;
            box-sizing: border-box;
            appearance: none;
            cursor: pointer;
            transition: border-color 90ms ease-in-out;

            &:hover {
                border-color: var(--c-blue-6);
            }

            &__caret {
                position: absolute;
                right: 26px;
                bottom: 20px;
                cursor: pointer;
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
            font-size: 15px;
            color: #354049;
            margin-bottom: 0;
        }
    }

</style>
