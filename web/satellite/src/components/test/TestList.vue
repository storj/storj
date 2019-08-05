// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <ListComponent
            :dataSet="getItems"
            :itemComponent="getChildComponent"
            :onItemClick="onChildItemClick"
        />
        <Button
                label="Add More"
                :onPress="onAddClick"/>
        <div>
            {{getSelectedValue}}
        </div>
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import Button from '@/components/common/Button.vue';
    import ListComponent from '@/components/common/ListComponent.vue';
    import Item from '@/components/test/Item.vue';

    @Component({
        components: {
            Button,
            ListComponent,
            Item
        }
    })
    export default class TestList extends Vue {
        private items: {}[] = [
                { 'a': 0 },
                { 'a': 1 },
                { 'a': 2 }
            ];

        private selectedTemplateItem: string = '';

        public get getItems(): {}[] {
            return this.items;
        }

        public get getSelectedValue():string {
            return this.selectedTemplateItem;
        }

        public get getChildComponent() {
            return Item;
        }

        public onChildItemClick(data: any) {
            this.selectedTemplateItem = data;
        }

        public onAddClick():void {
            this.items.push({a:Math.round(Math.random() * 100)});
        }
    }
</script>

<style scoped lang="scss">

</style>
