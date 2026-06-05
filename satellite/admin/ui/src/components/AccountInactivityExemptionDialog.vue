// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog v-model="model" width="auto" transition="fade-transition">
        <v-card
            rounded="xlg"
            :title="isGrant ? 'Grant Inactivity Exemption' : 'Revoke Inactivity Exemption'"
            :subtitle="isGrant
                ? 'Mark this account as exempt from inactivity-based suspension.'
                : 'Re-enable inactivity-based suspension checks for this account.'"
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
                            :placeholder="isGrant
                                ? 'Enter a reason for granting inactivity exemption.'
                                : 'Enter a reason for revoking inactivity exemption.'"
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
                            :color="isGrant ? 'primary' : 'warning'"
                            variant="flat"
                            :loading="isLoading"
                            :disabled="!reason"
                            block
                            @click="submit"
                        >
                            {{ isGrant ? 'Grant Exemption' : 'Revoke Exemption' }}
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
import { computed, ref, watch } from 'vue';

import { UserAccount } from '@/api/client.gen';
import { useLoading } from '@/composables/useLoading';
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

const isGrant = computed(() => !props.account.inactivityExempt);

function submit() {
    withLoading(async () => {
        try {
            await usersStore.toggleInactivityExemption(props.account.id, isGrant.value, reason.value);
            notify.success(isGrant.value ? 'Inactivity exemption granted.' : 'Inactivity exemption revoked.');
            await usersStore.updateCurrentUser(props.account.id);
            model.value = false;
        } catch (e) {
            const action = isGrant.value ? 'grant' : 'revoke';
            notify.error(`Failed to ${action} inactivity exemption. ${e.message}`);
        }
    });
}

watch(model, (newVal) => {
    if (newVal) reason.value = '';
});
</script>
