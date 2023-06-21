// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <table-item
        :class="{ 'owner': isProjectOwner }"
        :item="itemToRender"
        :selectable="true"
        :select-disabled="isProjectOwner"
        :selected="model.isSelected()"
        :on-click="(_) => $emit('memberClick', model)"
        @selectClicked="($event) => $emit('selectClicked', $event)"
    />
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { ProjectMember, ProjectMemberItemModel, ProjectRole } from '@/types/projectMembers';
import { useResize } from '@/composables/resize';
import { useProjectsStore } from '@/store/modules/projectsStore';

import TableItem from '@/components/common/TableItem.vue';

const { isMobile, isTablet } = useResize();
const projectsStore = useProjectsStore();

const props = withDefaults(defineProps<{
    model: ProjectMemberItemModel;
}>(), {
    model: () => new ProjectMember('', '', '', new Date(), ''),
});

const isProjectOwner = computed((): boolean => {
    return props.model.getUserID() === projectsStore.state.selectedProject.ownerId;
});

const itemToRender = computed((): { [key: string]: unknown } => {
    let role: ProjectRole = ProjectRole.Member;
    if (props.model.isPending()) {
        role = ProjectRole.Invited;
    } else if (isProjectOwner.value) {
        role = ProjectRole.Owner;
    }

    if (!isMobile.value && !isTablet.value) {
        const dateStr = props.model.getJoinDate().toLocaleDateString('en-US', { day:'numeric', month:'short', year:'numeric' });
        return {
            name: props.model.getName(),
            email: props.model.getEmail(),
            role: role,
            date: dateStr,
        };
    }

    if (isTablet.value) {
        return { name: props.model.getName(), email: props.model.getEmail(), role: role };
    }
    // TODO: change after adding actions button to list item
    return { name: props.model.getName(), email: props.model.getEmail() };
});
</script>

<style scoped lang="scss">
    :deep(.primary) {
        overflow: hidden;
        white-space: nowrap;
        text-overflow: ellipsis;
    }

    :deep(th) {
        max-width: 25rem;
    }

    @media screen and (width <= 940px) {

        :deep(th) {
            max-width: 10rem;
        }
    }
</style>
