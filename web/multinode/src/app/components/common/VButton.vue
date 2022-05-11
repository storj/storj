// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <!-- if isDisabled check onPress in parent element -->
    <div
        class="container"
        :class="containerClassName"
        :style="style"
        @click="onPress"
    >
        <svg v-if="withPlus" class="plus" xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 20 20" fill="none">
            <path d="M10 4.1665V15.8332" stroke="white" stroke-width="1.66667" stroke-linecap="round" stroke-linejoin="round" />
            <path d="M4.16797 10H15.8346" stroke="white" stroke-width="1.66667" stroke-linecap="round" stroke-linejoin="round" />
        </svg>
        <span class="label">{{ label }}</span>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

/**
 * Custom button component with label.
 */
// @vue/component
@Component
export default class VButton extends Vue {
    @Prop({ default: 'Default' })
    private readonly label: string;
    @Prop({ default: 'inherit' })
    private readonly width: string;
    @Prop({ default: '48px' })
    private readonly height: string;
    @Prop({ default: false })
    private readonly isWhite: boolean;
    @Prop({ default: false })
    private readonly isTransparent: boolean;
    @Prop({ default: false })
    private readonly isDeletion: boolean;
    @Prop({ default: false })
    private readonly isBlueWhite: boolean;
    @Prop({ default: false })
    private isDisabled: boolean;
    @Prop({ default: false })
    private withPlus: boolean;
    @Prop({ default: false })
    private inactive: boolean;
    @Prop({ default: () => () => {} })
    private readonly onPress: () => void;

    public get style(): Record<string, unknown> {
        return { width: this.width, height: this.height };
    }

    public get containerClassName(): string {
        let className = `${this.inactive ? 'inactive' : ''}`;

        switch (true) {
        case this.isDisabled:
            className = 'disabled';
            break;
        case this.isWhite:
            className = 'white';
            break;
        case this.isTransparent:
            className = 'transparent';
            break;
        case this.isDeletion:
            className = 'red';
        }

        return className;
    }
}
</script>

<style lang="scss" scoped>
    .container {
        display: flex;
        align-items: center;
        justify-content: center;
        background-color: var(--c-primary);
        border-radius: var(--br-button);
        cursor: pointer;

        .label {
            font-family: 'font_medium', sans-serif;
            font-size: 16px;
            color: var(--c-button-label);
            margin: 0;
        }

        &:hover {
            background-color: #004199;

            &.white {
                box-shadow: none !important;
                background-color: var(--c-button-white-hover) !important;
                border-color: transparent;
            }

            &.red {
                box-shadow: none !important;
                background-color: var(--c-button-red-hover);
            }
        }
    }

    .red {
        background-color: var(--c-button-red);
    }

    .plus {
        margin-right: 10px;
    }

    .white {
        background-color: var(--c-button-white);
        border: var(--b-button-white);

        .label {
            color: var(--c-button-white--label);
        }

        .plus {

            path {
                stroke: var(--c-title);
            }
        }
    }

    .disabled {
        background-color: var(--c-button-disabled);
        pointer-events: none !important;

        .label {
            color: #acb0bc !important;
        }
    }

    .inactive {
        opacity: 0.5;
        pointer-events: none !important;
    }
</style>
