// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <!-- if isDisabled check onPress in parent element -->
    <div
        :class="containerClassName"
        :style="style"
        @click="onPress">
        <h1 class="label" :class="{'white': isWhite}">{{label}}</h1>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

// Custom button component with label
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

        return 'container';
    }
}
</script>

<style scoped lang="scss">
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
        }

        .white,
        .red {
            background-color: transparent;
            border: 1px solid #afb7c1;

            .label {
                color: #354049;
            }
        }

        .disabled {
            background-color: #dadde5;
            border-color: #dadde5;

            .label {
                color: #acb0bc;
            }
        }

        &:hover {
            box-shadow: 0 4px 20px rgba(35, 121, 236, 0.4);

            &.white {
                box-shadow: none;
                background-color: #2683ff;
                border: 1px solid #2683ff;

                .label {
                    color: white;
                }
            }

            &.red {
                box-shadow: none;
                background-color: transparent;

                .label {
                    color: #eb5757;
                }
            }

            &.disabled {
                box-shadow: none;
                background-color: #dadde5 !important;

                .label {
                    color: #acb0bc !important;
                }
            }
        }
    }
</style>
