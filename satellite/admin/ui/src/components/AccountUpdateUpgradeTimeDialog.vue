// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <RequireReasonFormDialog
        v-model="model"
        :loading="isLoading"
        :initial-form-data="initialFormData"
        :form-config="formConfig"
        title="Update Upgrade Time"
        subtitle="Note: This action has impact on billing."
        width="600"
        @submit="update"
    />
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useDate } from 'vuetify/framework';

import { UpdateUserUpgradeTimeRequest, UserAccount } from '@/api/client.gen';
import { useLoading } from '@/composables/useLoading';
import { useUsersStore } from '@/store/users';
import { useNotify } from '@/composables/useNotify';
import { FieldType, FormConfig } from '@/types/forms';

import RequireReasonFormDialog from '@/components/RequireReasonFormDialog.vue';

const usersStore = useUsersStore();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();
const date = useDate();

const model = defineModel<boolean>({ required: true });

const props = defineProps<{
    account: UserAccount;
}>();

const formConfig: FormConfig = {
    sections: [{ rows: [
        { fields: [{
            key: 'upgradeTime',
            type: FieldType.Date,
            label: 'Upgrade Time',
            clearable: true,
            prependIcon: '',
            transform: {
                forward: (value) => value ? date.date(value): null,
                back: (value) => {
                    return value ? `${date.toISO(value)}T00:00:00Z` : null;
                },
            },
        }] },
    ] }],
};

const initialFormData = computed(() => ({
    upgradeTime: props.account.upgradeTime,
}));

function update(formData: Record<string, unknown>) {
    withLoading(async () => {
        const request = new UpdateUserUpgradeTimeRequest();
        for (const key in request) {
            if (!Object.hasOwn(formData, key)) continue;
            if (formData[key] === initialFormData.value[key]) continue;
            // set only changed fields
            request[key] = formData[key];
        }

        try {
            const account = await usersStore.updateUpgradeTime(props.account.id, request);
            await usersStore.updateCurrentUser(account);
            model.value = false;
            notify.success('Upgrade time updated successfully!');
        } catch (e) {
            notify.error(`Failed to update upgrade time. ${e.message}`);
        }
    });
}
</script>
