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
import { useStore } from '@/utils/hooks';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { ManageProjectPassphraseStep } from '@/types/managePassphrase';
import { MODALS } from '@/utils/constants/appStatePopUps';

import TableItem from '@/components/common/TableItem.vue';

const store = useStore();

/**
 * Open the modal for the current file.
 */
function openModal(): void {
    store.commit(APP_STATE_MUTATIONS.SET_MANAGE_PASSPHRASE_STEP, ManageProjectPassphraseStep.Switch);
    store.commit(APP_STATE_MUTATIONS.UPDATE_ACTIVE_MODAL, MODALS.manageProjectPassphrase);
}
</script>
