// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <table-item
        selectable
        select-hidden
        :on-click="onBack"
        :item="{'name': 'Back', 'size': '', 'date': ''}"
        item-type="back"
    >
        <template #options>
            <th />
        </template>
    </table-item>
</template>

<script setup lang="ts">
import TableItem from '@/components/common/TableItem.vue';

const props = defineProps<{
    onBack: () => void;
}>();

</script>
