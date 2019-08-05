// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <div v-for="item in dataSet">
            <component :is="itemComponent" :data="item" v-on:click.native="onItemClick(item)"></component>
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
        private readonly onItemClick: Function;
        @Prop({default: []})
        private readonly dataSet: T[];

    }
</script>

<style scoped lang="scss">
</style>
