// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <!-- if isDisabled check onPress in parent element -->
    <div
        :class="containerClassName"
        :style="style"
        @click="onPress">
        <span class="label">{{label}}</span>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

/**
 * Custom button component with label.
 */
@Component
export default class VButton extends Vue {
    @Prop({default: 'Default'})
    private readonly label: string;
    @Prop({default: 'inherit'})
    private readonly width: string;
    @Prop({default: 'inherit'})
    private readonly height: string;
    @Prop({default: false})
    private readonly isWhite: boolean;
    @Prop({default: false})
    private readonly isDeletion: boolean;
    @Prop({default: false})
    private readonly isBlueWhite: boolean;
    @Prop({default: false})
    private isDisabled: boolean;
    @Prop({default: () => { return; }})
    private readonly onPress: Function;

    public get style(): Object {
        return { width: this.width, height: this.height };
    }

    public get containerClassName(): string {
        if (this.isDisabled) return 'container disabled';

        if (this.isWhite) return 'container white';

        if (this.isDeletion) return 'container red';

        if (this.isBlueWhite) return 'container blue-white';

        return 'container';
    }
}
</script>

<style scoped lang="scss">
    .white,
    .red {
        background-color: transparent !important;
        border: 1px solid #afb7c1 !important;

        .label {
            color: #354049 !important;
        }
    }

    .blue-white {
        background-color: #fff !important;
        border: 2px solid #2683ff !important;

        .label {
            color: #2683ff !important;
        }
    }

    .disabled {
        background-color: #dadde5 !important;
        border-color: #dadde5 !important;

        .label {
            color: #acb0bc !important;
        }
    }

    .container {
        display: flex;
        align-items: center;
        justify-content: center;
        background-color: #2683ff;
        border-radius: 6px;
        cursor: pointer;

        .label {
            font-family: 'font_medium', sans-serif;
            font-size: 16px;
            line-height: 23px;
            color: #fff;
            margin: 0;
        }

        &:hover {
            background-color: #0059d0;

            &.white,
            &.blue-white {
                box-shadow: none !important;
                background-color: #2683ff !important;
                border: 1px solid #2683ff !important;

                .label {
                    color: white !important;
                }
            }

            &.blue-white {
                border: 2px solid #2683ff !important;
            }

            &.red {
                box-shadow: none !important;
                background-color: transparent !important;

                .label {
                    color: #eb5757 !important;
                }
            }

            &.disabled {
                box-shadow: none !important;
                background-color: #dadde5 !important;

                .label {
                    color: #acb0bc !important;
                }

                &:hover {
                    cursor: default;
                }
            }
        }
    }
</style>
