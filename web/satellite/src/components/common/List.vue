// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="item-component">
        <component
            v-for="item in dataSet"
            class="item-component__item"
            :is="itemComponent"
            :itemData="item"
            @click.native="onItemClick(item)"
            v-bind:class="[item.isSelected ? 'selected' : '']"
            v-bind:key="item.id"/>
    </div>
</template>

<script lang="ts">
    import { Component, Prop, Vue } from 'vue-property-decorator';

    @Component
    export default class List extends Vue {
        @Prop({default: ''})
        private readonly itemComponent: string;
        @Prop({
            default: () => {
                console.error('onItemClick is not reinitialized');
            }
        })
        private readonly onItemClick: ListItemClickCallback;
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
