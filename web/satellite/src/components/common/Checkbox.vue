// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <label class="container">
        <input type="checkbox" v-model="checked" @change="onChange">
        <span v-bind:class="[isCheckboxError ? 'checkmark error': 'checkmark']"></span>
    </label>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

// Custom checkbox component
@Component(
    {
        data: () => {
            return {
                checked: false
            };
        },
        methods: {
            // Emits data to parent component
            onChange() {
                this.$emit('setData', this.$data.checked);
            }
        },
        props: {
            isCheckboxError: {
                type: Boolean,
                default: false
            },
        },
    }
)
export default class Checkbox extends Vue {

}
</script>

<style scoped lang="scss">
    .container {
        display: block;
        position: relative;
        padding-left: 20px;
        height: 25px;
        width: 25px;
        cursor: pointer;
        font-size: 22px;
        -webkit-user-select: none;
        -moz-user-select: none;
        -ms-user-select: none;
        user-select: none;
        outline: none;
    }

    .container input {
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

    .container:hover input ~ .checkmark {
        background-color: #ccc;
    }

    .container input:checked ~ .checkmark {
        border: 2px solid #2196F3;
        background-color: #2196F3;
    }

    .checkmark:after {
        content: "";
        position: absolute;
        display: none;
    }

    .checkmark.error {
        border-color: red;
    }

    .container input:checked ~ .checkmark:after {
        display: block;
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
</style>
