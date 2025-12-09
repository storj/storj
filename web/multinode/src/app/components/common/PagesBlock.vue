// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="pages-container">
        <span
            v-for="page in pages"
            :key="page.index"
            class="pages-container__pages"
            :class="{ 'selected': isSelected(page.index) }"
            @click="page.select()"
        >{{ page.index }}</span>
    </div>
</template>

<script setup lang="ts">
import { CheckSelected, Page } from '@/app/types/pagination';

withDefaults(defineProps<{
    pages?: Page[];
    isSelected?: CheckSelected;
}>(), {
    pages: () => [],
    isSelected: () => false,
});
</script>

<style scoped lang="scss">
    .pages-container {
        display: flex;

        &__pages {
            font-family: 'font_medium', sans-serif;
            font-size: 16px;
            margin-right: 8px;
            text-align: center;
            cursor: pointer;
            position: relative;
            border: 1px solid #e1e3e6;
            border-radius: 6px;
            width: 40px;
            height: 40px;
            display: flex;
            align-items: center;
            justify-content: center;
            transition: all 0.2s ease;

            &:hover {
                color: #2379ec;
                border-color: #2379ec;
            }

            &:last-child {
                margin-right: 0;
            }
        }
    }

    .selected {
        color: #2379ec;
        font-family: 'font_bold', sans-serif;
        border-color: #2379ec;
    }
</style>
