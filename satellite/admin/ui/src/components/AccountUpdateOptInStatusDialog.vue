// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <RequireReasonFormDialog
        v-model="model"
        :loading="isLoading"
        :initial-form-data="initialFormData"
        :form-config="formConfig"
        title="Update Opt-In Status"
        subtitle="Set status to 'No Action' or 'Excluded'"
        width="600"
        @submit="update"
    />
</template>

<script setup lang="ts">
import { computed, watch } from 'vue';

import { UpdateUserOptInStatusRequest, UserAccount } from '@/api/client.gen';
import { useLoading } from '@/composables/useLoading';
import { useUsersStore } from '@/store/users';
import { useNotify } from '@/composables/useNotify';
import { FieldType, FormConfig } from '@/types/forms';

import RequireReasonFormDialog from '@/components/RequireReasonFormDialog.vue';

const usersStore = useUsersStore();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const model = defineModel<boolean>({ required: true });

const props = defineProps<{
    account: UserAccount;
}>();

const formConfig = computed<FormConfig>(() => ({
    sections: [{ rows: [
        { fields: [{
            key: 'status',
            type: FieldType.Select,
            label: 'Opt-In Status',
            items: usersStore.state.optInStatuses,
            itemTitle: 'name',
            itemValue: 'value',
            prependIcon: '',
        }] },
    ] }],
}));

watch(model, (open) => {
    if (open) usersStore.getOptInStatuses();
});

const initialFormData = computed(() => ({
    status: props.account.optInStatus.value,
}));

function update(formData: Record<string, unknown>) {
    withLoading(async () => {
        const status = formData.status as number;
        if (status === initialFormData.value.status) {
            notify.warning('Opt-in status unchanged.');
            model.value = false;
            return;
        }

        const request = new UpdateUserOptInStatusRequest();
        request.status = status;
        request.reason = formData.reason as string;

        try {
            await usersStore.updateOptInStatus(props.account.id, request);
            await usersStore.updateCurrentUser(props.account.id);
            model.value = false;
            notify.success('Opt-in status updated successfully!');
        } catch (e) {
            notify.error(`Failed to update opt-in status. ${e.message}`);
        }
    });
}
</script>
