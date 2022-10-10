// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="container">
        <TopArrowIcon :class="`${ isActive && isTop?'active':'inactive' }`" />
        <BottomArrowIcon :class="`${ isActive && isBottom?'active':'inactive' }`" />
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { SortingDirectionEnum } from '@/types/sortingArrows';

import BottomArrowIcon from '@/../static/images/common/bottomArrow.svg';
import TopArrowIcon from '@/../static/images/common/topArrow.svg';

// @vue/component
@Component({
    components: {
        TopArrowIcon,
        BottomArrowIcon,
    },
})
export default class VerticalArrows extends Vue {
    @Prop({ default: false })
    private isActive: boolean;
    @Prop({ default: SortingDirectionEnum.BOTTOM })
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

    .inactive {
        opacity: 0.5;
    }

    .active {
        opacity: 1;

        :deep(path) {
            fill: #2683ff !important;
        }
    }

</style>
