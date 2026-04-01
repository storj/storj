// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <RequireReasonFormDialog
        v-model="model"
        :loading="isLoading"
        :initial-form-data="initialFormData"
        :form-config="formConfig"
        title="Update Tenant ID"
        width="600"
        @submit="update"
    />
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { UpdateUserTenantIDRequest, UserAccount } from '@/api/client.gen';
import { useLoading } from '@/composables/useLoading';
import { useUsersStore } from '@/store/users';
import { useAppStore } from '@/store/app';
import { useNotify } from '@/composables/useNotify';
import { FieldType, FormConfig } from '@/types/forms';

import RequireReasonFormDialog from '@/components/RequireReasonFormDialog.vue';

const usersStore = useUsersStore();
const appStore = useAppStore();

const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const model = defineModel<boolean>({ required: true });

const props = defineProps<{
    account: UserAccount;
}>();

const tenantIDOptions = computed<Record<string, string>[]>(() => {
    const list = appStore.state.settings.console.tenantIDList;
    if (!(list && list.length)) return [];

    return list.map(i => ({ name: i, value: i }));
});

const formConfig: FormConfig = {
    sections: [{ rows: [
        { fields: [{
            key: 'tenantID',
            type: FieldType.Select,
            label: 'Tenant ID',
            placeholder: 'Select Tenant ID',
            items: tenantIDOptions.value,
            itemTitle: 'name',
            itemValue: 'value',
            clearable: true,
            prependIcon: '',
        }] },
    ] }],
};

const initialFormData = computed(() => ({
    tenantID: props.account.tenantID,
}));

function update(formData: Record<string, unknown>) {
    withLoading(async () => {
        const request = new UpdateUserTenantIDRequest();
        for (const key in request) {
            if (!Object.hasOwn(formData, key)) continue;
            if (formData[key] === initialFormData.value[key]) continue;
            // set only changed fields
            request[key] = formData[key];
        }

        try {
            const account = await usersStore.updateTenantID(props.account.id, request);
            await usersStore.updateCurrentUser(account);
            model.value = false;
            notify.success('Tenant ID updated successfully!');
        } catch (e) {
            notify.error(`Failed to update tenant ID. ${e.message}`);
        }
    });
}
</script>
