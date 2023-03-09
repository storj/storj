// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="pages-container">
        <span
            v-for="page in pages"
            :key="page.index"
            class="pages-container__pages"
            :class="{'selected': isSelected(page.index)}"
            @click="page.select()"
        >{{ page.index }}</span>
    </div>
</template>

<script setup lang="ts">
import { CheckSelected, Page } from '@/types/pagination';

const props = withDefaults(defineProps<{
    isSelected: CheckSelected;
    pages: Page[];
}>(), {
    pages: () => [],
});
</script>

<style scoped lang="scss">
    .pages-container {
        display: flex;

        &__pages {
            font-family: 'font_medium', sans-serif;
            font-size: 16px;
            margin-right: 15px;
            width: auto;
            text-align: center;
            cursor: pointer;
            display: block;
            position: relative;
            transition: all 0.2s ease;

            &:hover {
                color: #2379ec;

                &:after {
                    content: '';
                    display: block;
                    position: absolute;
                    bottom: -4px;
                    left: 0;
                    width: 100%;
                    height: 2px;
                    background-color: #2379ec;
                }
            }

            &:last-child {
                margin-right: 0;
            }
        }
    }

    .selected {
        color: #2379ec;
        font-family: 'font_bold', sans-serif;

        &:after {
            content: '';
            display: block;
            position: absolute;
            bottom: -4px;
            left: 0;
            width: 100%;
            height: 2px;
            background-color: #2379ec;
        }
    }
</style>
