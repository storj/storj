// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div
        class="dropdown"
        @click.self="toggleOptions"
        :class="{ active: areOptionsShown }"
    >
        <span class="label">{{ selectedOption.label }}</span>
        <div class="dropdown__selection" v-show="areOptionsShown">
            <div class="dropdown__selection__overflow-container">
                <div v-for="option in allOptions" :key="option.label" class="dropdown__selection__option" @click="onOptionClick(option)">
                    <span class="dropdown__selection__option__label">{{ option.label }}</span>
                </div>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue, Watch } from 'vue-property-decorator';

export class Option {
    public constructor(
        public label: string = '',
        public onClick: Function = () => { return; },
    ) {}
}

@Component
export default class VDropdown extends Vue {
    @Prop({default: 'All'})
    private readonly allLabel: string;
    @Prop({default: () => { return; }})
    private readonly onAllClick: Function;
    @Prop({default: []})
    private readonly options: Option[];

    public areOptionsShown: boolean = false;

    @Watch('options')
    public allOptions: Option[] = [ new Option(this.allLabel, this.onAllClick), ...this.options ];

    @Watch('options')
    public selectedOption: Option = this.allOptions[0];

    public toggleOptions(): void {
        this.areOptionsShown = !this.areOptionsShown;
    }

    public closeOptions(): void {
        this.areOptionsShown = false;
    }

    /**
     * Fires on option click.
     * Calls callback and changes selection.
     * @param option
     */
    public async onOptionClick(option: Option): Promise<void> {
        await option.onClick();
        this.selectedOption = option;
        this.closeOptions();
    }
}
</script>

<style lang="scss">
    .dropdown {
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
                white-space: nowrap;
                text-overflow: ellipsis;
                overflow: hidden;
                width: 100% !important;
                cursor: pointer;
                border-bottom: 1px solid var(--c-gray--light);

                &:hover {
                    background: var(--c-background);
                }
            }
        }
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
