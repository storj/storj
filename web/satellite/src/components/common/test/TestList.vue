// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <List
        :data-set="dataSetItems"
        :item-component="getItemComponent"
        :on-item-click="onItemClick"/>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import TestListItem from '@/components/common/test/TestListItem.vue';
import List from '@/components/common/VList.vue';

@Component({
    components: {
        List,
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

<style scoped lang="scss"></style>

