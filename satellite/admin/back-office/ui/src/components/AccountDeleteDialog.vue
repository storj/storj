// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog v-model="model" width="auto" transition="fade-transition">
        <v-card rounded="xlg">
            <template #title>
                Delete Account
            </template>
            <template #append>
                <v-btn
                    :icon="X" :disabled="isLoading"
                    variant="text" size="small" color="default" @click="model = false"
                />
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

                <v-alert class="mt-6" title="Warning" variant="tonal" color="error" rounded="lg">
                    This will delete the account, data, and account
                    information.
                </v-alert>
            </div>

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn variant="outlined" color="default" block @click="model = false">Cancel</v-btn>
                    </v-col>
                    <v-col>
                        <v-btn color="error" variant="flat" block :loading="isLoading" @click="deleteAccount">Delete Account</v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import {
    VDialog,
    VCard,
    VBtn,
    VRow,
    VCol,
    VTextField,
    VCardActions,
    VAlert,
} from 'vuetify/components';
import { X } from 'lucide-vue-next';
import { useRouter } from 'vue-router';

import { useLoading } from '@/composables/useLoading';
import { UserAccount } from '@/api/client.gen';
import { useUsersStore } from '@/store/users';
import { useNotify } from '@/composables/useNotify';
import { ROUTES } from '@/router';

const notify = useNotify();
const usersStore = useUsersStore();
const router = useRouter();
const { isLoading, withLoading } = useLoading();

const model = defineModel<boolean>({ required: true });

const props = defineProps<{
    account: UserAccount;
}>();

function deleteAccount() {
    withLoading(async () => {
        try {
            await usersStore.deleteUser(props.account.id);
            notify.success('Account deleted successfully');

            model.value = false;
            await new Promise((resolve) => setTimeout(resolve, 200)); // wait for dialog to close
            router.push({ name: ROUTES.AccountSearch.name });
        } catch (e) {
            notify.error(`Failed to delete account. ${e.message}`);
        }
    });
}
</script>
