// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <label class="container">
        <input class="checkmark-input" type="checkbox" v-model="checked" @change="onChange">
        <span class="checkmark" :class="{'error': isCheckboxError}"></span>
    </label>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

// Custom checkbox component
@Component
export default class VCheckbox extends Vue {
    @Prop({default: false})
    private readonly isCheckboxError: boolean;

    private checked: boolean = false;

    /**
     * Emits value to parent component.
     */
    public onChange(): void {
        this.$emit('setData', this.checked);
    }
}
</script>

<style scoped lang="scss">
    .container {
        display: block;
        position: relative;
        padding-left: 20px;
        height: 23px;
        width: 23px;
        cursor: pointer;
        font-size: 22px;
        -webkit-user-select: none;
        -moz-user-select: none;
        -ms-user-select: none;
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
        height: 25px;
        width: 25px;
        border: 2px solid rgba(56, 75, 101, 0.4);
        border-radius: 4px;
    }

    .checkmark:after {
        content: '';
        position: absolute;
        display: none;
    }

    .container .checkmark:after {
        left: 9px;
        top: 5px;
        width: 5px;
        height: 10px;
        border: solid white;
        border-width: 0 3px 3px 0;
        -webkit-transform: rotate(45deg);
        -ms-transform: rotate(45deg);
        transform: rotate(45deg);
    }

    .container:hover .checkmark-input ~ .checkmark {
        background-color: #ccc;
    }

    .container .checkmark-input:checked ~ .checkmark {
        border: 2px solid #2196f3;
        background-color: #2196f3;
    }

    .checkmark.error {
        border-color: red;
    }

    .container .checkmark-input:checked ~ .checkmark:after {
        display: block;
    }
</style>
