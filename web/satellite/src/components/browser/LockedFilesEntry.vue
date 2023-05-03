// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <table-item
        selectable
        select-hidden
        :on-click="openModal"
        :on-primary-click="openModal"
        :item="{'name': 'Objects locked', 'size': '', 'date': ''}"
        item-type="locked"
    >
        <th slot="options" />
    </table-item>
</template>

<script setup lang="ts">
import { ManageProjectPassphraseStep } from '@/types/managePassphrase';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useAppStore } from '@/store/modules/appStore';

import TableItem from '@/components/common/TableItem.vue';

const appStore = useAppStore();

/**
 * Open the modal for the current file.
 */
function openModal(): void {
    appStore.setManagePassphraseStep(ManageProjectPassphraseStep.Switch);
    appStore.updateActiveModal(MODALS.manageProjectPassphrase);
}
</script>
