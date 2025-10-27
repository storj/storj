// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <RequireReasonFormDialog
        v-model="model"
        :loading="isLoading"
        :initial-form-data="initialFormData"
        :form-config="formConfig"
        title="Update Account Default Limits"
        subtitle="Enter default limits per project for this account"
        width="600"
        @submit="update"
    />
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { useUsersStore } from '@/store/users';
import { useNotify } from '@/composables/useNotify';
import { useLoading } from '@/composables/useLoading';
import { UpdateUserRequest, UserAccount } from '@/api/client.gen';
import { FieldType, FormConfig, rawNumberField, terabyteFormField } from '@/types/forms';

import RequireReasonFormDialog from '@/components/RequireReasonFormDialog.vue';

const usersStore = useUsersStore();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const model = defineModel<boolean>({ required: true });

const props = defineProps<{
    account: UserAccount;
}>();

const initialFormData = computed(() => ({
    projectLimit: props.account?.projectLimit ?? 0,
    segmentLimit: props.account?.segmentLimit ?? 0,
    storageLimit: props.account?.storageLimit ?? 0,
    bandwidthLimit: props.account?.bandwidthLimit ?? 0,
    email: props.account?.email ?? 0,
}));

const formConfig = computed((): FormConfig => {
    return {
        sections: [
            {
                rows: [
                    {
                        fields: [
                            rawNumberField({ key: 'projectLimit', label: 'Total projects',
                                cols:{ default: 12, sm: 6 },
                            }),
                            rawNumberField({ key: 'segmentLimit', label: 'Segments / project',
                                step: 5000,
                                cols:{ default: 12, sm: 6 },
                            }),
                        ],
                    }, {
                        fields: [
                            terabyteFormField({ key: 'storageLimit', label: 'Storage (TB) / project',
                                cols: { default: 12, sm: 6 },
                            }),
                            terabyteFormField({ key: 'bandwidthLimit', label: 'Download (TB) / month / project',
                                cols: { default: 12, sm: 6 },
                            }),
                        ],
                    }, {
                        fields: [
                            {
                                key: 'email',
                                type: FieldType.Text,
                                label: 'Account Email',
                                readonly: true,
                            },
                        ],
                    },
                ],
            },
        ],
    };
});

function update(formData: Record<string, unknown>) {
    withLoading(async () => {
        const request = new UpdateUserRequest();
        for (const key in request) {
            if (!Object.hasOwn(formData, key)) continue;
            // set only changed fields
            if (formData[key] === initialFormData.value[key]) continue;
            request[key] = formData[key];
        }

        try {
            const account = await usersStore.updateUser(props.account.id, request);
            await usersStore.updateCurrentUser(account);

            model.value = false;
            notify.success('Limits updated successfully!');
        } catch (e) {
            notify.error(`Failed to update limits. ${e.message}`);
        }
    });
}
</script>