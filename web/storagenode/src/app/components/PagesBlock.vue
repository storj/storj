// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="pages-container">
        <span
            class="pages-container__pages"
            v-for="page in pages"
            :class="{'selected': isSelected(page.index)}"
            @click="page.select()"
            :key="page.index"
        >
            {{ page.index }}
        </span>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { CheckSelected, Page } from '@/app/types/pagination';

@Component
export default class PagesBlock extends Vue {
    @Prop({default: () => []})
    public readonly pages: Page[];
    @Prop({default: () => false})
    public readonly isSelected: CheckSelected;
}
</script>

<style scoped lang="scss">
    .pages-container {
        display: flex;
        user-select: none;

        &__pages {
            font-family: 'font_medium', sans-serif;
            font-size: 16px;
            margin-right: 15px;
            width: 10px;
            text-align: center;
            cursor: pointer;
            display: block;
            position: relative;
            transition: all 0.2s ease;
            color: var(--page-number-color);

            &:hover {
                color: var(--link-color);

                &:after {
                    content: '';
                    display: block;
                    position: absolute;
                    bottom: -4px;
                    left: 0;
                    width: 100%;
                    height: 2px;
                    background-color: var(--link-color);
                }
            }

            &:last-child {
                margin-right: 0;
            }
        }
    }

    .selected {
        color: var(--link-color);
        font-family: 'font_bold', sans-serif;

        &:after {
            content: '';
            display: block;
            position: absolute;
            bottom: -4px;
            left: 0;
            width: 10px;
            height: 2px;
            background-color: var(--link-color);
        }
    }
</style>
