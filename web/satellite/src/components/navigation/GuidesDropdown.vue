// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div ref="dropdown" v-click-outside="close" class="side-dropdown" :style="style">
        <slot />
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

// @vue/component
@Component
export default class GuidesDropdown extends Vue {
    @Prop({ default: 0 })
    private readonly yPosition: number;
    @Prop({ default: 0 })
    private readonly xPosition: number;
    @Prop({ default: () => () => {} })
    private readonly close: () => void;

    private dropdownMiddle = 0;

    /**
     * Mounted hook after initial render.
     * Calculates dropdowns Y middle point.
     */
    public mounted(): void {
        this.dropdownMiddle = this.$refs.dropdown.getBoundingClientRect().height / 2;
    }

    public $refs!: {
        dropdown: HTMLDivElement;
    };

    /**
     * Returns top and left position of dropdown.
     */
    public get style(): Record<string, string> {
        return { top: `${this.yPosition - this.dropdownMiddle}px`, left: `${this.xPosition}px` };
    }
}
</script>

<style scoped lang="scss">
    .side-dropdown {
        position: absolute;
        background: #fff;
        border: 1px solid #ebeef1;
        box-shadow: 0 0 20px rgb(0 0 0 / 4%);
        border-radius: 8px;
        width: 390px;
        z-index: 1;
    }
</style>
