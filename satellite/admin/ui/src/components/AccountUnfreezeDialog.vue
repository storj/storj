// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog v-model="model" width="auto" transition="fade-transition">
        <v-card
            rounded="xlg"
            title="Unfreeze Account"
            subtitle="Enter a reason for unfreezing this account"
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
                            placeholder="Enter reason for unfreezing this account"
                            variant="solo-filled"
                            hide-details="auto"
                            autofocus
                            flat
                        />
                    </v-col>
                </v-row>
            </div>

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn variant="outlined" color="default" block @click="model = false">Cancel</v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            variant="flat"
                            :loading="isLoading"
                            :disabled="!reason"
                            block
                            @click="unfreezeAccount"
                        >
                            Unfreeze Account
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { VBtn, VCard, VCardActions, VCol, VDialog, VRow, VTextarea, VTextField } from 'vuetify/components';
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

const props = defineProps<{
    account: UserAccount;
}>();

const reason = ref('');

function unfreezeAccount() {
    withLoading(async () => {
        try {
            await usersStore.unfreezeUser(props.account.id, reason.value);
            await usersStore.updateCurrentUser(props.account.id);
            notify.success('Account unfrozen successfully.');
            model.value = false;
        } catch (e) {
            notify.error(`Failed to unfreeze account. ${e.message}`);
        }
    });
}

watch(model, (newVal) => {
    if (newVal) reason.value = '';
});
</script>
