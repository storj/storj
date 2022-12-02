// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <table-item
        :item="itemToRender"
        :on-click="onClick"
        class="container__item"
    />
</template>

<script lang="ts">
import { Component, Prop } from 'vue-property-decorator';

import { Project } from '@/types/projects';

import TableItem from '@/components/common/TableItem.vue';
import Resizable from '@/components/common/Resizable.vue';

// @vue/component
@Component({
    components: {
        TableItem,
    },
})
export default class ProjectsListItem extends Resizable {
    @Prop({ default: () => new Project('123', 'name', 'desc') })
    private readonly itemData: Project;
    @Prop({ default: () => (_: string) => {} })
    public readonly onClick: (project: string) => void;

    public get itemToRender(): { [key: string]: string | string[] } {
        if (!this.isMobile) return { name: this.itemData.name, memberCount: this.itemData.memberCount.toString(), date: this.itemData.createdDate() };

        return { info: [ this.itemData.name, `Created ${this.itemData.createdDate()}` ] };
    }
}
</script>

<style scoped lang="scss">
    .container {

        &__item {
            width: 33%;
            font-family: 'font_regular', sans-serif;
            font-size: 16px;
            margin: 0;
        }
    }

</style>
