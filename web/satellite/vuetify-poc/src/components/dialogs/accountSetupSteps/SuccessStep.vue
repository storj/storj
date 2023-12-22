// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container class="fill-height" fluid>
        <v-row justify="center" align="center">
            <v-col class="text-center py-10">
                <icon-blue-checkmark />
                <p class="text-overline mt-4 mb-2">
                    Account Complete
                </p>
                <h2 class="mb-3">You are now ready to use Storj</h2>
                <p>Create your first project, and upload files to share with the world.</p>
                <p>Let us know if you need any help getting started!</p>
                <v-btn
                    class="mt-7"
                    size="large"
                    :append-icon="mdiChevronRight"
                    :loading="isLoading"
                    @click="finishSetup()"
                >
                    Continue
                </v-btn>
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import { VBtn, VCol, VContainer, VRow } from 'vuetify/components';
import { mdiChevronRight } from '@mdi/js';

import { useUsersStore } from '@/store/modules/usersStore';
import { useNotify } from '@/utils/hooks';
import { useLoading } from '@/composables/useLoading';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

import IconBlueCheckmark from '@poc/components/icons/IconBlueCheckmark.vue';

const userStore = useUsersStore();

const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const emit = defineEmits<{
    continue: [];
}>();

function finishSetup() {
    withLoading(async () => {
        try {
            await userStore.getUser();
            emit('continue');
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.ACCOUNT_SETUP_DIALOG);
        }
    });
}
</script>