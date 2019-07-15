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
    import { Component, Vue, Prop } from 'vue-property-decorator';

    // Custom button component with label
    @Component
    export default class Button extends Vue {
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
        // TODO: improve default implementation
        @Prop({default: () => console.error('onPress is not reinitialized')})
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
                background-color: #DADDE5 !important;

                .label {
                    color: #ACB0BC !important;
                }
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
        border-color: #DADDE5;
        .label {
            color: #ACB0BC;
        }
    }
</style>
