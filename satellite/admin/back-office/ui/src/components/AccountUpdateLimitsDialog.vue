// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog v-model="model" width="550" transition="fade-transition">
        <v-card rounded="xlg">
            <template #title>
                Update Account Default Limits
            </template>
            <template #subtitle>
                Enter default limits per project for this account.
            </template>
            <template #append>
                <v-btn
                    :icon="X" :disabled="isLoading"
                    variant="text" size="small" color="default" @click="model = false"
                />
            </template>

            <v-form v-model="valid" :disabled="isLoading" @submit.prevent="update">
                <div class="pa-6">
                    <DynamicFormBuilder
                        ref="formBuilder"
                        :config="formConfig"
                        :initial-data="initialFormData"
                    />
                </div>

                <v-card-actions class="pa-6">
                    <v-row>
                        <v-col>
                            <v-btn
                                variant="outlined"
                                color="default" block
                                :disabled="isLoading"
                                @click="model = false"
                            >
                                Cancel
                            </v-btn>
                        </v-col>
                        <v-col>
                            <v-btn
                                color="primary"
                                variant="flat"
                                block
                                type="submit"
                                :disabled="!valid || !hasFormChanged"
                                :loading="isLoading"
                                @click="update"
                            >
                                Update
                            </v-btn>
                        </v-col>
                    </v-row>
                </v-card-actions>
            </v-form>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { VBtn, VCard, VDialog, VForm, VRow, VCol, VCardActions } from 'vuetify/components';
import { X } from 'lucide-vue-next';

import { useUsersStore } from '@/store/users';
import { useNotify } from '@/composables/useNotify';
import { useLoading } from '@/composables/useLoading';
import { UpdateUserRequest, UserAccount } from '@/api/client.gen';
import { FieldType, FormBuilderExpose, FormConfig, rawNumberField, terabyteFormField } from '@/types/forms';

import DynamicFormBuilder from '@/components/form-builder/DynamicFormBuilder.vue';

const usersStore = useUsersStore();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const model = defineModel<boolean>({ required: true });

const props = defineProps<{
    account: UserAccount;
}>();

const valid = ref(false);

const formBuilder = ref<FormBuilderExpose>();

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

const hasFormChanged = computed(() => {
    const formData = formBuilder.value?.getData() as Record<string, unknown> | undefined;
    if (!formData) return false;

    for (const key in initialFormData.value) {
        if (formData[key] !== initialFormData.value[key]) {
            return true;
        }
    }
    return false;
});

function update() {
    if (!valid.value) {
        return;
    }

    withLoading(async () => {
        const request = new UpdateUserRequest();
        const formData = formBuilder.value?.getData() || {};
        if (!formData) return;

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

watch(model, (shown) => {
    if (!shown) return;
    formBuilder.value?.reset();
});
</script>