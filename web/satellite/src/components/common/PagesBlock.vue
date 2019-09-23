// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="pages-container">
        <span
            v-for="page in pages"
            :class="{'selected': isSelected(page.index)}"
            @click="page.select()"
            :key="page.index">{{page.index}}</span>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { Page } from '@/types/pagination';

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
    }

    .selected {
        color: #2379EC;
        font-family: 'font_bold';

        &:after {
            content: '';
            display: block;
            position: absolute;
            bottom: -4px;
            left: 0;
            width: 10px;
            height: 2px;
            background-color: #2379EC;
        }
    }

    span {
        font-family: 'font_medium';
        font-size: 16px;
        margin-right: 15px;
        width: 10px;
        text-align: center;
        cursor: pointer;
        display: block;
        position: relative;
        transition: all .2s ease;

        &:hover {
            color: #2379EC;

            &:after {
                content: '';
                display: block;
                position: absolute;
                bottom: -4px;
                left: 0;
                width: 100%;
                height: 2px;
                background-color: #2379EC;
            }
        }

        &:last-child {
            margin-right: 0;
        }
    }
</style>
