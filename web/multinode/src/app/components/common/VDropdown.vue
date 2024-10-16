// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div
        v-if="options.length"
        class="dropdown"
        :class="{ active: areOptionsShown }"
        @click.stop="toggleOptions"
    >
        <span class="label">{{ selectedOption.label }}</span>
        <svg width="8" height="4" viewBox="0 0 8 4" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path d="M3.33657 3.73107C3.70296 4.09114 4.29941 4.08814 4.66237 3.73107L7.79796 0.650836C8.16435 0.291517 8.01864 0 7.47247 0L0.526407 0C-0.0197628 0 -0.16292 0.294525 0.200917 0.650836L3.33657 3.73107Z" fill="currentColor" />
        </svg>
        <div v-if="areOptionsShown" v-click-outside="closeOptions" class="dropdown__selection">
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
        public onClick: OptionClick = async(_id) => Promise.resolve(),
    ) {}
}

// @vue/component
@Component
export default class VDropdown extends Vue {
    @Prop({ default: [] })
    private readonly options: Option[];

    @Prop({ default: null })
    private readonly preselectedOption: Option;

    public areOptionsShown = false;

    public selectedOption: Option;

    public created(): void {
        this.selectedOption = this.preselectedOption || this.options[0];
    }

    public toggleOptions(): void {
        this.areOptionsShown = !this.areOptionsShown;
    }

    public closeOptions(): void {
        if (!this.areOptionsShown) { return; }

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
        width: 100%;
        max-width: 300px;
        height: 40px;
        background: transparent;
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 0 16px;
        border: 1px solid var(--v-border-base);
        border-radius: 6px;
        font-size: 16px;
        color: var(--v-text-base);
        cursor: pointer;
        font-family: 'font_medium', sans-serif;
        z-index: 998;

        &:hover {
            border-color: var(--c-gray);
        }

        &.active {
            border-color: var(--c-primary);
        }

        &__selection {
            position: absolute;
            top: 52px;
            left: 0;
            width: 100%;
            border: 1px solid var(--v-border-base);
            border-radius: 6px;
            overflow: hidden;
            background: var(--v-background-base);
            z-index: 999;

            &__overflow-container {
                overflow: overlay;
                overflow-x: hidden;
                max-height: 160px;
            }

            &__option {
                display: flex;
                align-items: center;
                justify-content: flex-start;
                padding: 0 16px;
                height: 40px;
                width: 100% !important;
                cursor: pointer;
                border-bottom: 1px solid var(--v-border-base);
                box-sizing: border-box;

                &:hover {
                    background: var(--v-active-base);
                }
            }
        }
    }

    .label {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        margin-right: 5px;
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
