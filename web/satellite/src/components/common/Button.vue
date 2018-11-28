// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <!-- if isDisabled check onPress in parent element -->
    <div 
        v-bind:class="containerClassName" 
        :style="style" 
        v-on:click="onPress"> 
            <h1 v-bind:class="[isWhite ? 'label white' : 'label']">{{label}}</h1>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

// Custom button component with label
@Component(
    { 
		props: {
            label: {
                type: String,
                default: 'Default'
            },
            width: {
                type: String,
                default: 'inherit'
            },
            height: {
                type: String,
                default: 'inherit'
            },
            isWhite: {
                type: Boolean,
                default: false
            },
            isDisabled: {
                type: Boolean,
                default: false
            },
            onPress: {
                type: Function,
                default: () => {}
            }
        },
        computed: {
            style: function () {
                return { width: this.$props.width, height: this.$props.height }
            },
            containerClassName: function () {
                if (this.$props.isDisabled) {
                    return 'container disabled';
                }

                return this.$props.isWhite ? 'container white' : 'container';
            }
        }
    }
)

export default class Button extends Vue {}
</script>

<style scoped lang="scss">
    .container {
        display: flex;
        align-items: center;
        justify-content: center;
        background-color: #2683FF;
        border-radius: 6px;
        cursor: pointer;

        .label {
            font-family: 'montserrat_medium';
			font-size: 16px;
			line-height: 23px;
            color: #fff;
        }

        .label.white {
            color: #354049;
        }
    }
    .container.white {
        background-color: transparent;
        border: 1px solid #AFB7C1;
    }
    .container.disabled {
        background-color: #DADDE5;
        .label.white {
            color: #ACB0BC;
        }
    }
</style>
