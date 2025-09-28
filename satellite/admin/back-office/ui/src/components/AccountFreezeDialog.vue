// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog v-model="model" width="400" transition="fade-transition">
        <v-card rounded="xlg">
            <template #title>
                Freeze Account
            </template>
            <template #subtitle>
                Select the freeze type to apply.
            </template>
            <template #append>
                <v-btn :icon="X" variant="text" size="small" color="default" @click="model = false" />
            </template>

            <v-form v-model="valid" :disabled="isLoading" @submit.prevent="freezeAccount">
                <div class="pa-6">
                    <DynamicFormBuilder
                        ref="formBuilder"
                        :config="formConfig"
                        :initial-data="initialFormData"
                    />
                </div>
            </v-form>

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn
                            variant="outlined" color="default"
                            :disabled="isLoading"
                            block
                            @click="model = false"
                        >
                            Cancel
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            color="warning" variant="flat"
                            :loading="isLoading"
                            :disabled="!valid"
                            block
                            @click="freezeAccount"
                        >
                            Freeze Account
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { VBtn, VCard, VCardActions, VCol, VDialog, VForm, VRow } from 'vuetify/components';
import { X } from 'lucide-vue-next';

import { UserAccount } from '@/api/client.gen';
import { useUsersStore } from '@/store/users';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { FieldType, FormBuilderExpose, FormConfig } from '@/types/forms';
import { RequiredRule } from '@/types/common';

import DynamicFormBuilder from '@/components/form-builder/DynamicFormBuilder.vue';

const usersStore = useUsersStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const model = defineModel<boolean>({ required: true });

const props = defineProps<{
    account: UserAccount;
}>();

const valid = ref(false);
const formBuilder = ref<FormBuilderExpose>();

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

function freezeAccount() {
    withLoading(async () => {
        try {
            await usersStore.freezeUser(props.account.id, formBuilder.value?.getData().freezeType as number);
            await usersStore.updateCurrentUser(props.account.id);
            notify.success('Account frozen successfully.');
            model.value = false;
        } catch (error) {
            notify.error(`Failed to freeze account. ${error.message}`);
            return;
        }
    });
}

watch(() => model.value, (newVal) => {
    if (!newVal) return;
    formBuilder.value?.reset();
});
</script>