// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <table-item
        :class="{ 'owner': isProjectOwner }"
        :item="{ name: itemData.name, date: itemData.localDate(), email: itemData.email }"
        :selectable="true"
        :select-disabled="isProjectOwner"
        :selected="itemData.isSelected"
        :on-click="(_) => $emit('memberClick', itemData)"
        @selectChange="(value) => $emit('selectChange', value)"
    />
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { ProjectMember } from '@/types/projectMembers';
import TableItem from "@/components/common/TableItem.vue";

// @vue/component
@Component({
    components: {TableItem}
})
export default class ProjectMemberListItem extends Vue {
    @Prop({ default: new ProjectMember('', '', '', new Date(), '') })
    public itemData: ProjectMember;

    public get isProjectOwner(): boolean {
        return this.itemData.user.id === this.$store.getters.selectedProject.ownerId;
    }
}
</script>

<style scoped lang="scss">
    .owner {
        cursor: not-allowed;

        & > ::v-deep th:nth-child(2):after {
            content: 'Project Owner';
            font-size: 13px;
            color: #afb7c1;
        }
    }
</style>
