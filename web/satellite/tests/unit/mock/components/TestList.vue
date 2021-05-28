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

import VList from '@/components/common/VList.vue';

import TestListItem from './TestListItem.vue';

@Component({
    components: {
        VList,
    },
})
export default class TestList extends Vue {
    @Prop({
        default: () => {
            console.error('onItemClick is not initialized');
        },
    })
    private readonly onItemClick: (item: any) => Promise<void>;

    private items: string[] = ['1', '2', '3'];

    public get getItemComponent() {
        return TestListItem;
    }

    public get dataSetItems(): string[] {
        return this.items;
    }
}
</script>
