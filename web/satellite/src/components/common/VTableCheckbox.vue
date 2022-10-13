// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <label class="container" @click.stop> <!--don't propagate click event to parent <tr>-->
        <input
            id="checkbox" :disabled="disabled" :checked="value"
            class="checkmark-input"
            type="checkbox" @change="onChange"
        >
        <span class="checkmark" />
    </label>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

// Custom checkbox alternative to VCheckbox.vue for use in TableItem.vue
// this has no label and allows for external toggles
// @vue/component
@Component
export default class VTableCheckbox extends Vue {
    @Prop({ default: false })
    private readonly value: boolean;
    @Prop({ default: false })
    private readonly disabled: boolean;

    /**
     * Emits value to parent component.
     */
    public onChange(event: { target: {checked: boolean} }): void {
        this.$emit('checkChange', event.target.checked);
    }
}
</script>

<style scoped lang="scss">
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
