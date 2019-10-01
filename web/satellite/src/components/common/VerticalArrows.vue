// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="container">
        <svg :class="{ active: isActive && isTop }"
            width="9" height="6" viewBox="0 0 9 6" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path class="arrow-svg-path" d="M4.73684 5.70565e-07L9 6L-9.53674e-07 6L4.73684 5.70565e-07Z" fill="#354049"/>
        </svg>
        <svg :class="{ active: isActive && isBottom }"
            width="9" height="6" viewBox="0 0 9 6" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path class="arrow-svg-path" d="M4.26316 6L1.90735e-06 0L9 1.59559e-06L4.26316 6Z" fill="#354049"/>
        </svg>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { SortingDirectionEnum } from '@/types/sortingArrows';

@Component
export default class VerticalArrows extends Vue {
    @Prop({default: false})
    private isActive: boolean;
    @Prop({default: SortingDirectionEnum.BOTTOM})
    private direction: SortingDirectionEnum;

    public get isTop(): boolean {
        return this.direction === SortingDirectionEnum.TOP;
    }

    public get isBottom(): boolean {
        return this.direction === SortingDirectionEnum.BOTTOM;
    }
}
</script>

<style scoped lang="scss">
    .container {
        display: flex;
        flex-direction: column;
        margin-left: 10px;
        justify-content: space-between;
        height: 17px;
    }

    .active {

        .arrow-svg-path {
            fill: #2683FF !important;
        }
    }
</style>
