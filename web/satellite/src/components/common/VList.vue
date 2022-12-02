// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="item-component">
        <component
            :is="itemComponent"
            v-for="(item, key) in dataSet"
            :key="key"
            class="item-component__item"
            :item-data="item"
            :class="{ selected: item.isSelected }"
            @click.native="onItemClick(item)"
            @altMethod="openDeleteModal"
        />
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

declare type listItemClickCallback = (item: unknown) => Promise<void>;

// @vue/component
@Component
export default class VList extends Vue {
    @Prop({ default: '' })
    private readonly itemComponent: string;
    @Prop({ default: () => () => new Promise(() => false) })
    private readonly onItemClick: listItemClickCallback;
    @Prop({ default: [] })
    private readonly dataSet: unknown[];

    /**
     * testMethod
     */
    public openDeleteModal() {
        this.$emit('openModal');
    }
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
