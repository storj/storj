// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog v-model="model" width="auto" transition="fade-transition">
        <v-card rounded="xlg">
            <template #title>
                Disable MFA
            </template>
            <template #subtitle>
                <span class="text-wrap">
                    Disable multi-factor-authentication for this account?
                </span>
            </template>
            <template #append>
                <v-btn :icon="X" variant="text" size="small" color="default" @click="model = false" />
            </template>

            <div class="pa-6">
                <v-row>
                    <v-col cols="12">
                        <v-text-field
                            :model-value="account.id" label="Account ID" variant="solo-filled" flat readonly
                            hide-details="auto"
                        />
                    </v-col>
                    <v-col cols="12">
                        <v-text-field
                            :model-value="account.email" label="Account Email" variant="solo-filled" flat readonly
                            hide-details="auto"
                        />
                    </v-col>
                </v-row>
            </div>

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn
                            variant="outlined"
                            color="default"
                            block :disabled="isLoading"
                            @click="model = false"
                        >
                            Cancel
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn color="primary" variant="flat" block :loading="isLoading" @click="disableMFA">Disable MFA</v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { VBtn, VCard, VCardActions, VCol, VDialog, VRow, VTextField } from 'vuetify/components';
import { X } from 'lucide-vue-next';

import { UserAccount } from '@/api/client.gen';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { useUsersStore } from '@/store/users';

const usersStore = useUsersStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const model = defineModel<boolean>({ required: true });

const props = defineProps<{
    account: UserAccount;
}>();

function disableMFA() {
    withLoading(async () => {
        try {
            await usersStore.disableMFA(props.account.id);
            const account = { ...props.account };

            account.mfaEnabled = false;
            usersStore.updateCurrentUser(account);
            notify.success('Multi-factor-authentication disabled');
            model.value = false;
        } catch (e) {
            notify.error(`Failed to disable multi-factor-authentication ${e.message}`);
        }
    });
}
</script>