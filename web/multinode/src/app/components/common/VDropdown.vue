// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div
        class="dropdown"
        @click.stop="toggleOptions"
        :class="{ active: areOptionsShown }"
        v-if="options.length"
    >
        <span class="label">{{ selectedOption.label }}</span>
        <div class="dropdown__selection" v-if="areOptionsShown" v-click-outside="closeOptions">
            <div class="dropdown__selection__overflow-container">
                <div v-for="option in options" :key="option.label" class="dropdown__selection__option" @click="onOptionClick(option)">
                    <span class="label">{{ option.label }}</span>
                </div>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

/**
 * OptionClick defines on click callback type for VDropdown Option.
 */
export type OptionClick = (id?: string) => Promise<void>;

/**
 * Option is a representation of VDropdown item.
 */
export class Option {
    public constructor(
        public label: string = 'no options',
        public onClick: OptionClick = async (id) => Promise.resolve(),
    ) {}
}

@Component
export default class VDropdown extends Vue {
    @Prop({default: []})
    private readonly options: Option[];

    public areOptionsShown: boolean = false;

    public selectedOption: Option;

    public created(): void {
        this.selectedOption = this.options[0];
    }

    public toggleOptions(): void {
        this.areOptionsShown = !this.areOptionsShown;
    }

    public closeOptions(): void {
        if (!this.areOptionsShown) return;

        this.areOptionsShown = false;
    }

    /**
     * Fires on option click.
     * Calls callback and changes selection.
     * @param option
     */
    public async onOptionClick(option: Option): Promise<void> {
        this.selectedOption = option;
        await option.onClick();
        this.closeOptions();
    }
}
</script>

<style lang="scss">
    .dropdown {
        position: relative;
        box-sizing: border-box;
        width: 300px;
        height: 40px;
        background: transparent;
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 0 16px;
        border: 1px solid var(--c-gray--light);
        border-radius: 6px;
        font-size: 16px;
        color: var(--c-title);
        cursor: pointer;
        font-family: 'font_medium', sans-serif;

        &:hover {
            border-color: var(--c-gray);
            color: var(--c-title);
        }

        &.active {
            border-color: var(--c-primary);
        }

        &__selection {
            position: absolute;
            top: 52px;
            left: 0;
            width: 300px;
            border: 1px solid var(--c-gray--light);
            border-radius: 6px;
            overflow: hidden;
            background: white;
            z-index: 999;

            &__overflow-container {
                overflow: overlay;
                overflow-x: hidden;
                height: 160px;
            }

            &__option {
                display: flex;
                align-items: center;
                justify-content: flex-start;
                padding: 0 16px;
                height: 40px;
                width: 100% !important;
                cursor: pointer;
                border-bottom: 1px solid var(--c-gray--light);

                &:hover {
                    background: var(--c-background);
                }
            }
        }
    }

    .label {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    ::-webkit-scrollbar {
        width: 3px;
    }

    ::-webkit-scrollbar-track {
        box-shadow: inset 0 0 5px transparent;
    }

    ::-webkit-scrollbar-thumb {
        background: var(--c-gray--light);
        border-radius: 6px;
        height: 5px;
    }
</style>
