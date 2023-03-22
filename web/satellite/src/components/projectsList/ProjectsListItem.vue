// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <table-item
        :item="itemToRender"
        :on-click="onClick"
        class="container__item"
    />
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { Project } from '@/types/projects';
import { useResize } from '@/composables/resize';

import TableItem from '@/components/common/TableItem.vue';

const props = withDefaults(defineProps<{
    itemData: Project,
    onClick: (project: string) => void,
}>(), {
    itemData: () => new Project('default', 'name', 'desc'),
    onClick: (_: string) => {},
});

const { isMobile } = useResize();

const itemToRender = computed((): { [key: string]: string | string[] } => {
    if (!isMobile.value) {
        return {
            name: props.itemData.name,
            memberCount: props.itemData.memberCount.toString(),
            date: props.itemData.createdDate(),
        };
    }

    return { info: [ props.itemData.name, `Created ${props.itemData.createdDate()}` ] };
});
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
