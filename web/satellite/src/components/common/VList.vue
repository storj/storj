// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="item-component">
        <component
            v-for="(item, key) in dataSet"
            class="item-component__item"
            :is="itemComponent"
            :item-data="item"
            @click.native="onItemClick(item)"
            :class="{ selected: item.isSelected }"
            :key="key"
        />
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

declare type listItemClickCallback = (item: any) => Promise<void>;

@Component
export default class VList extends Vue {
    @Prop({default: ''})
    private readonly itemComponent: string;
    @Prop({default: () => new Promise(() => false)})
    private readonly onItemClick: listItemClickCallback;
    @Prop({default: Array()})
    private readonly dataSet: any[];
}
</script>

<style scoped lang="scss">
    .item-component {
        width: 100%;

        &__item {
            width: 100%;
        }
    }
</style>
