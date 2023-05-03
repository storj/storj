// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <label class="container" @click.stop.prevent="selectClicked"> <!--don't propagate click event to parent <tr>-->
        <input
            id="checkbox" :disabled="disabled" :checked="value"
            class="checkmark-input"
            type="checkbox"
        >
        <span class="checkmark" />
    </label>
</template>

<script setup lang="ts">
const props = withDefaults(defineProps<{
    value?: boolean,
    disabled?: boolean,
}>(), { value: false, disabled: false });

const emit = defineEmits(['selectClicked']);

/**
 * Emits click event to parent component.
 * The parent is responsible for toggling the value prop.
 */
function selectClicked(event: Event): void {
    emit('selectClicked', event);
}
</script>

<style scoped lang="scss">
    .container {
        position: relative;
        display: flex;
        align-items: center;
        justify-content: center;
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
        height: 20px;
        width: 20px;
        border: 1px solid var(--c-grey-4);
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

    .container .checkmark-input:disabled ~ .checkmark {
        background-color: #f2eeee;
    }

    .container .checkmark-input:checked ~ .checkmark:after {
        display: block;
    }

    .label {
        cursor: pointer;
        font-size: 14px;
    }
</style>
