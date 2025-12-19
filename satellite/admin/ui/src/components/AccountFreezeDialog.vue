// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <RequireReasonFormDialog
        v-model="model"
        :loading="isLoading"
        :initial-form-data="initialFormData"
        :form-config="formConfig"
        title="Freeze Account"
        subtitle="Select the freeze type to apply"
        width="400"
        @submit="freezeAccount"
    />
</template>

<script setup lang="ts">
import { computed, watch } from 'vue';

import { UserAccount } from '@/api/client.gen';
import { useUsersStore } from '@/store/users';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { FieldType, FormConfig } from '@/types/forms';
import { RequiredRule } from '@/types/common';

import RequireReasonFormDialog from '@/components/RequireReasonFormDialog.vue';

const usersStore = useUsersStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const model = defineModel<boolean>({ required: true });

const props = defineProps<{
    account: UserAccount;
}>();

const freezeTypes = computed(() => usersStore.state.freezeTypes);

const initialFormData = computed(() => ({ freezeType: null }));

const formConfig = computed((): FormConfig => {
    return {
        sections: [
            {
                rows: [
                    {
                        fields: [
                            {
                                key: 'freezeType',
                                type: FieldType.Select,
                                label: 'Freeze type',
                                placeholder: 'Select freeze type',
                                items: freezeTypes.value,
                                itemTitle: 'name',
                                itemValue: 'value',
                                rules: [RequiredRule],
                                required: true,
                            },
                        ],
                    },
                ],
            },
        ],
    };
});

function freezeAccount(formData: Record<string, unknown>) {
    withLoading(async () => {
        try {
            await usersStore.freezeUser(props.account.id, formData.freezeType as number, formData.reason as string);
            await usersStore.updateCurrentUser(props.account.id);
            notify.success('Account frozen successfully.');
            model.value = false;
        } catch (error) {
            notify.error(`Failed to freeze account. ${error.message}`);
            return;
        }
    });
}

watch(model, (_) => usersStore.getAccountFreezeTypes());
</script>