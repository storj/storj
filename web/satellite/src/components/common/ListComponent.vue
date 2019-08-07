// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="item-component">
        <div v-for="item in dataSet">
            <component class="item-component__item"
                :is="itemComponent"
                :itemData="item"
                v-on:click.native="onItemClick(item)"
                v-bind:class="[item.isSelected ? 'selected' : '']"/>
        </div>
    </div>
</template>

<script lang="ts">
    import { Component, Prop, Vue } from 'vue-property-decorator';

    @Component({
        components: {},
    })
    export default class ListComponent<T> extends Vue {
        @Prop({default: ''})
        private readonly itemComponent: string;
        @Prop({
            default: () => {
                console.error('onItemClick is not reinitialized');
            }
        })
        private readonly onItemClick: ListItemClickCallback<T>;
        @Prop({default: []})
        private readonly dataSet: T[];

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
