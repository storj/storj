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
        @selectChange="(value) => $emit('selectChange', value)"
    />
</template>

<script lang="ts">
import { Component, Prop } from 'vue-property-decorator';

import { ProjectMember } from '@/types/projectMembers';

import TableItem from '@/components/common/TableItem.vue';
import Resizable from '@/components/common/Resizable.vue';

// @vue/component
@Component({
    components: { TableItem },
})
export default class ProjectMemberListItem extends Resizable {
    @Prop({ default: new ProjectMember('', '', '', new Date(), '') })
    public itemData: ProjectMember;

    public get isProjectOwner(): boolean {
        return this.itemData.user.id === this.$store.getters.selectedProject.ownerId;
    }

    public get itemToRender(): { [key: string]: string | string[] } {
        if (!this.isMobile && !this.isTablet) return { name: this.itemData.name, date: this.itemData.localDate(), email: this.itemData.email };

        // TODO: change after adding actions button to list item
        return { name: this.itemData.name, email: this.itemData.email };
    }
}
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
</style>
