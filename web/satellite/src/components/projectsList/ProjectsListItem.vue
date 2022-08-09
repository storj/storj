// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <table-item
        :item="{ name: itemData.name, memberCount: itemData.memberCount, date: itemData.createdDate() }"
        :on-click="onClick"
        class="container__item"
    />
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { Project } from '@/types/projects';
import TableItem from "@/components/common/TableItem.vue";

// @vue/component
@Component({
    components: {
        TableItem,
    },
})
export default class ProjectsListItem extends Vue {
    @Prop({default: () => new Project('123', 'name', 'desc')})
    private readonly itemData: Project;
    @Prop({ default: () => (_: string) => {} })
    public readonly onClick: (project: string) => void;
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
