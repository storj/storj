// Copyright (C) 2019 Storj Labs, Inc.
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
            isDeletion: {
                type: Boolean,
                default: false
            },
            isDisabled: {
                type: Boolean,
                default: false
            },
            onPress: {
                type: Function,
                default: () => {
                    console.error('onPress is not reinitialized');
                }
            }
        },
        computed: {
            style: function () {
                return {width: this.$props.width, height: this.$props.height};
            },
            containerClassName: function () {
                if (this.$props.isDisabled) return 'container disabled';

                if (this.$props.isWhite) return 'container white';

                if (this.$props.isDeletion) return 'container red';
                
                return 'container';
            },
        }
    }
)

export default class Button extends Vue {
}
</script>

<style scoped lang="scss">
    .container {
        display: flex;
        align-items: center;
        justify-content: center;
        background-color: #2683FF;
        border-radius: 6px;
        cursor: pointer;

        &:hover {
            box-shadow: 0px 4px 20px rgba(35, 121, 236, 0.4);

            &.white {
                box-shadow: none;
                background-color: #2683FF;
                border: 1px solid #2683FF;

                .label {
                    color: white;
                }
            }

            &.red {
                box-shadow: none;
                background-color: transparent;

                .label {
                    color: #EB5757;
                }
            }

            &.disabled {
                box-shadow: none;
            }
        }

        .label {
            font-family: 'font_medium';
			font-size: 16px;
			line-height: 23px;
            color: #fff;
        }
    }
    .container.white,
    .container.red {
        background-color: transparent;
        border: 1px solid #AFB7C1;

        .label {
            color: #354049;
        }
    }
    .container.disabled {
        background-color: #DADDE5;
        .label.white {
            color: #ACB0BC;
        }
    }
</style>
