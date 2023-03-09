// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="wrap">
        <label class="container">
            <input id="checkbox" v-model="checked" class="checkmark-input" type="checkbox" @change="onChange">
            <span class="checkmark" :class="{'error': isCheckboxError}" />
        </label>
        <label v-if="label" class="label" for="checkbox">{{ label }}</label>
    </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';

const props = withDefaults(defineProps<{
    isCheckboxError?: boolean;
    label?: string;
}>(), {
    isCheckboxError: false,
    label: '',
});

const emit = defineEmits(['setData']);

const checked = ref<boolean>(false);

/**
 * Emits value to parent component.
 */
function onChange(): void {
    emit('setData', checked.value);
}
</script>

<style scoped lang="scss">
    .wrap {
        display: flex;
        align-items: center;
        width: 100%;
    }

    .container {
        display: block;
        position: relative;
        padding-left: 15px;
        height: 20px;
        width: 20px;
        cursor: pointer;
        user-select: none;
        outline: none;
    }

    .container .checkmark-input {
        position: absolute;
        opacity: 0;
        cursor: pointer;
        height: 0;
        width: 0;
    }

    .checkmark {
        position: absolute;
        top: 0;
        left: 0;
        height: 20px;
        width: 20px;
        border: 2px solid rgb(56 75 101 / 40%);
        border-radius: 4px;
        box-sizing: border-box;
    }

    .checkmark:after {
        content: '';
        position: absolute;
        display: none;
    }

    .container .checkmark:after {
        left: 4px;
        top: 0;
        width: 5px;
        height: 10px;
        border: solid white;
        border-width: 0 3px 3px 0;
        transform: rotate(45deg);
    }

    .container:hover .checkmark-input ~ .checkmark {
        background-color: #ccc;
    }

    .container .checkmark-input:checked ~ .checkmark {
        border: 2px solid #376fff;
        background-color: #376fff;
    }

    .checkmark.error {
        border-color: red;
    }

    .container .checkmark-input:checked ~ .checkmark:after {
        display: block;
    }

    .label {
        cursor: pointer;
        font-size: 14px;
    }
</style>
