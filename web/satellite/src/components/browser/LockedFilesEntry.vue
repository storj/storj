// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <table-item
        :on-click="openModal"
        :on-primary-click="openModal"
        :item="{'name': 'Objects locked', 'size': '', 'date': ''}"
        table-type="locked"
    >
        <th slot="options" />
    </table-item>
</template>

<script setup lang="ts">
import { useStore } from '@/utils/hooks';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { ManageProjectPassphraseStep } from '@/types/managePassphrase';

import TableItem from '@/components/common/TableItem.vue';

const store = useStore();

/**
 * Open the modal for the current file.
 */
function openModal(): void {
    store.commit(APP_STATE_MUTATIONS.TOGGLE_MANAGE_PROJECT_PASSPHRASE_MODAL_SHOWN, ManageProjectPassphraseStep.Switch);
}
</script>
