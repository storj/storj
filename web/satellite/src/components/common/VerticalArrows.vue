// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="container">
        <TopArrowIcon :class="`${ isActive && isTop?'active':'inactive' }`" />
        <BottomArrowIcon :class="`${ isActive && isBottom?'active':'inactive' }`" />
    </div>
</template>

<script setup lang="ts">

import { computed } from 'vue';

import { SortingDirectionEnum } from '@/types/sortingArrows';

import BottomArrowIcon from '@/../static/images/common/bottomArrow.svg';
import TopArrowIcon from '@/../static/images/common/topArrow.svg';

const props = withDefaults(defineProps<{
    isActive?: boolean;
    direction?: SortingDirectionEnum;
}>(), {
    isActive: false,
    direction: SortingDirectionEnum.BOTTOM,
});

const isTop = computed((): boolean => props.direction === SortingDirectionEnum.TOP);
const isBottom = computed((): boolean => props.direction === SortingDirectionEnum.BOTTOM);
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
