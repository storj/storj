// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="container">
        <TopArrowIcon :class="{ active: isActive && isTop }"/>
        <BottomArrowIcon :class="{ active: isActive && isBottom }"/>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import BottomArrowIcon from '@/../static/images/common/bottomArrow.svg';
import TopArrowIcon from '@/../static/images/common/topArrow.svg';

import { SortingDirectionEnum } from '@/types/sortingArrows';

@Component({
    components: {
        TopArrowIcon,
        BottomArrowIcon,
    },
})
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
            fill: #2683ff !important;
        }
    }
</style>
