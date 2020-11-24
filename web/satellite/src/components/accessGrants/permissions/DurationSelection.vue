// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="duration-selection">
        <div
            class="duration-selection__toggle-container"
            @click="togglePicker"
        >
            <h1 class="duration-selection__toggle-container__name">Forever</h1>
            <ExpandIcon
                class="duration-selection__toggle-container__expand-icon"
                alt="Arrow down (expand)"
            />
        </div>
        <DurationPicker
            v-if="isDurationPickerVisible"
        />
    </div>

</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import DurationPicker from '@/components/accessGrants/permissions/DurationPicker.vue';

import ExpandIcon from '@/../static/images/common/BlackArrowExpand.svg';

@Component({
    components: {
        ExpandIcon,
        DurationPicker,
    },
})

export default class DurationSelection extends Vue {
    public isDurationPickerVisible: boolean = false;
    /**
     * Toggles duration picker.
     */
    public togglePicker(): void {
        this.isDurationPickerVisible = !this.isDurationPickerVisible;
    }
    /**
     * Closes duration picker.
     */
    public closePicker(): void {
        this.isDurationPickerVisible = false;
    }
}
</script>

<style scoped lang="scss">
    .duration-selection {
        background-color: #fff;
        cursor: pointer;
        margin-left: 15px;
        border-radius: 6px;
        border: 1px solid rgba(56, 75, 101, 0.4);
        font-family: 'font_regular', sans-serif;
        width: 235px;
        position: relative;

        &__toggle-container {
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 15px 20px;
            width: calc(100% - 40px);

            &__name {
                font-style: normal;
                font-weight: normal;
                font-size: 16px;
                line-height: 21px;
                color: #384b65;
                margin: 0;
            }
        }
    }
</style>
