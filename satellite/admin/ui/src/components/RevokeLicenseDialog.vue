// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog v-model="model" width="600" transition="fade-transition">
        <v-card
            rounded="xlg"
            title="Revoke License"
            subtitle="Confirm license revocation"
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
                        <v-alert type="warning" variant="tonal" density="compact" class="mb-2">
                            This action will set the revocation timestamp. The license will no longer be active and cannot be undone.
                        </v-alert>
                    </v-col>
                    <v-col v-if="license" cols="12">
                        <p class="text-body-2 mb-1"><strong>License Type:</strong> {{ license.type }}</p>
                        <p class="text-body-2 mb-1"><strong>Project:</strong> {{ license.publicId || 'All Projects' }}</p>
                        <p class="text-body-2 mb-1"><strong>Bucket:</strong> {{ license.bucketName || 'All Buckets' }}</p>
                        <p class="text-body-2 mb-1"><strong>Expires:</strong> {{ date.format(license.expiresAt, 'fullDateTime') }}</p>
                    </v-col>
                    <v-col cols="12">
                        <v-textarea
                            v-model="reason"
                            :rules="[RequiredRule]"
                            label="Reason"
                            placeholder="Enter reason for revoking this license"
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
                            color="error"
                            :loading="isLoading"
                            :disabled="!reason"
                            block
                            @click="revokeLicense"
                        >
                            Revoke License
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { VAlert, VBtn, VCard, VCardActions, VCol, VDialog, VRow, VTextarea } from 'vuetify/components';
import { X } from 'lucide-vue-next';
import { ref, watch } from 'vue';
import { useDate } from 'vuetify';

import { UserLicense } from '@/api/client.gen';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { useUsersStore } from '@/store/users';
import { RequiredRule } from '@/types/common';

const notify = useNotify();
const usersStore = useUsersStore();
const { isLoading, withLoading } = useLoading();
const date = useDate();

const model = defineModel<boolean>({ required: true });

const props = defineProps<{
    userId: string;
    license: UserLicense | null;
}>();

const emit = defineEmits<{
    success: [];
}>();

const reason = ref('');

function revokeLicense() {
    const license = props.license;
    if (!license) return;

    withLoading(async () => {
        try {
            await usersStore.revokeUserLicense(props.userId, {
                type: license.type,
                publicId: license.publicId || undefined,
                bucketName: license.bucketName || undefined,
                expiresAt: license.expiresAt,
                reason: reason.value,
            });
            notify.success('License revoked successfully');
            model.value = false;
            emit('success');
        } catch (error) {
            notify.error('Failed to revoke license', error);
        }
    });
}

watch(model, (newVal) => {
    if (newVal) reason.value = '';
});
</script>
