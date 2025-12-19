// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog v-model="model" width="auto" transition="fade-transition">
        <v-card
            rounded="xlg"
            :title="markPendingDeletion ? 'Mark Pending Deletion': 'Delete Account'"
            :subtitle="`Enter a reason for ${ markPendingDeletion ? 'marking' : 'deleting'} this account ${ markPendingDeletion ? 'for deletion' : '' }`"
        >
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
                            :model-value="account.id"
                            label="Account ID"
                            variant="solo-filled"
                            hide-details="auto"
                            flat readonly
                        />
                    </v-col>
                    <v-col cols="12">
                        <v-text-field
                            :model-value="account.email"
                            label="Account Email"
                            variant="solo-filled"
                            hide-details="auto"
                            flat readonly
                        />
                    </v-col>
                    <v-col cols="12">
                        <v-textarea
                            v-model="reason"
                            :rules="[RequiredRule]"
                            label="Reason"
                            :placeholder="`Enter a reason for ${ markPendingDeletion ? 'marking' : 'deleting'} this account ${ markPendingDeletion ? 'for deletion' : '' }.`"
                            variant="solo-filled"
                            hide-details="auto"
                            autofocus
                            flat
                        />
                    </v-col>
                </v-row>

                <v-alert class="mt-6" title="Warning" variant="tonal" color="error" rounded="lg">
                    <template v-if="markPendingDeletion">
                        This will set status to "<strong>Pending Deletion</strong>".
                        <br>
                        The account will be deleted later by a chore.
                    </template>
                    <template v-else>
                        This will delete the account, data, and account
                        information.
                    </template>
                </v-alert>
            </div>

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn variant="outlined" color="default" block @click="model = false">Cancel</v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            color="error" variant="flat"
                            :loading="isLoading"
                            :disabled="!reason"
                            block
                            @click="deleteAccount"
                        >
                            {{ markPendingDeletion ? 'Mark Pending Deletion' : 'Delete Account' }}
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { VAlert, VBtn, VCard, VCardActions, VCol, VDialog, VRow, VTextarea, VTextField } from 'vuetify/components';
import { X } from 'lucide-vue-next';
import { ref, watch } from 'vue';

import { useLoading } from '@/composables/useLoading';
import { UserAccount } from '@/api/client.gen';
import { useUsersStore } from '@/store/users';
import { useNotify } from '@/composables/useNotify';
import { RequiredRule } from '@/types/common';

const notify = useNotify();
const usersStore = useUsersStore();
const { isLoading, withLoading } = useLoading();

const model = defineModel<boolean>({ required: true });
const markPendingDeletion = defineModel<boolean>('markPendingDeletion', { default: false });

const props = defineProps<{
    account: UserAccount;
}>();

const reason = ref('');

function deleteAccount() {
    withLoading(async () => {
        try {
            const user = await usersStore.deleteUser(props.account.id, markPendingDeletion.value, reason.value);
            await usersStore.updateCurrentUser(user);

            notify.success(`Account ${markPendingDeletion.value ? 'marked for deletion' : 'deleted'} successfully`);
            model.value = false;
        } catch (e) {
            notify.error(e);
        }
    });
}

watch(model, (newVal) => {
    if (!newVal && markPendingDeletion.value) markPendingDeletion.value = false;
    if (newVal) reason.value = '';
});
</script>
