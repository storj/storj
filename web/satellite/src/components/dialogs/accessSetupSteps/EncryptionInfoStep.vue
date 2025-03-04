// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-row class="pa-4 ma-0">
        <v-col cols="12">
            By generating S3 credentials, you are opting in to
            <a class="link" href="https://docs.storj.io/dcs/concepts/encryption-key/design-decision-server-side-encryption/" target="_blank" rel="noopener noreferrer">
                server-side encryption.
            </a>
        </v-col>
        <v-col cols="12">
            <v-checkbox
                density="compact"
                label="I understand, don't show this again."
                color="default"
                hide-details
                @update:model-value="value => toggleServerSideEncryptionNotice(value as boolean)"
            />
        </v-col>
    </v-row>
</template>

<script setup lang="ts">
import { VRow, VCol, VCheckbox } from 'vuetify/components';

import { useUsersStore } from '@/store/modules/usersStore';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/composables/useNotify';

const userStore = useUsersStore();

const notify = useNotify();

async function toggleServerSideEncryptionNotice(value: boolean): Promise<void> {
    try {
        const noticeDismissal = { ...userStore.state.settings.noticeDismissal };
        noticeDismissal.serverSideEncryption = value;
        await userStore.updateSettings({ noticeDismissal });
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.ACCESS_GRANTS_PAGE);
    }
}
</script>
