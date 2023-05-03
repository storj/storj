// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <table-item
        :class="{ 'owner': isProjectOwner }"
        :item="itemToRender"
        :selectable="true"
        :select-disabled="isProjectOwner"
        :selected="itemData.isSelected"
        :on-click="(_) => $emit('memberClick', itemData)"
        @selectClicked="($event) => $emit('selectClicked', $event)"
    />
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { ProjectMember } from '@/types/projectMembers';
import { useResize } from '@/composables/resize';
import { useProjectsStore } from '@/store/modules/projectsStore';

import TableItem from '@/components/common/TableItem.vue';

const { isMobile, isTablet } = useResize();
const projectsStore = useProjectsStore();

const props = withDefaults(defineProps<{
    itemData: ProjectMember;
}>(), {
    itemData: () => new ProjectMember('', '', '', new Date(), ''),
});

const isProjectOwner = computed((): boolean => {
    return props.itemData.user.id === projectsStore.state.selectedProject.ownerId;
});

const itemToRender = computed((): { [key: string]: string | string[] } => {
    if (!isMobile.value && !isTablet.value) return { name: props.itemData.name, date: props.itemData.localDate(), email: props.itemData.email };

    // TODO: change after adding actions button to list item
    return { name: props.itemData.name, email: props.itemData.email };
});
</script>

<style scoped lang="scss">
    .owner {
        cursor: not-allowed;

        & > :deep(th:nth-child(2):after) {
            content: 'Project Owner';
            font-size: 13px;
            color: #afb7c1;
        }
    }

    :deep(.primary) {
        overflow: hidden;
        white-space: nowrap;
        text-overflow: ellipsis;
    }

    :deep(th) {
        max-width: 25rem;
    }

    @media screen and (max-width: 940px) {

        :deep(th) {
            max-width: 10rem;
        }
    }
</style>
