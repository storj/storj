// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VList
        :data-set="dataSetItems"
        :item-component="getItemComponent"
        :on-item-click="onItemClick"
    />
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import TestListItem from './TestListItem.vue';

import VList from '@/components/common/VList.vue';

// @vue/component
@Component({
    components: {
        VList,
    },
})
export default class TestList extends Vue {
    @Prop({
        default: () => () => {
            console.error('onItemClick is not initialized');
        },
    })
    private readonly onItemClick: (item: unknown) => Promise<void>;

    private items: string[] = ['1', '2', '3'];

    public get getItemComponent(): typeof TestListItem {
        return TestListItem;
    }

    public get dataSetItems(): string[] {
        return this.items;
    }
}
</script>
