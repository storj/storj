// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <table-item
        :item="{ name: itemData.name, date: itemData.localDate() }"
        :selectable="true"
        :selected="itemData.isSelected"
        :on-click="(_) => $emit('accessGrantClick', itemData)"
        @selectChange="(value) => $emit('selectChange', value)"
    />
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { AccessGrant } from '@/types/accessGrants';

import TableItem from '@/components/common/TableItem.vue';

// @vue/component
@Component({
    components: {
        TableItem,
    },
})
export default class AccessGrantsItem extends Vue {
    @Prop({ default: new AccessGrant('', '', new Date(), '') })
    private readonly itemData: AccessGrant;
}
</script>
