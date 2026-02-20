// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        max-width="420px"
        transition="fade-transition"
        :persistent="isLoading"
    >
        <v-card>
            <v-card-item class="pa-6">
                <template #prepend>
                    <v-sheet
                        class="border-sm d-flex justify-center align-center"
                        width="40"
                        height="40"
                        rounded="lg"
                    >
                        <component :is="Pencil" :size="18" />
                    </v-sheet>
                </template>
                <v-card-title class="font-weight-bold">Rename Access Grant</v-card-title>
                <template #append>
                    <v-btn
                        :icon="X"
                        variant="text"
                        size="small"
                        color="default"
                        :disabled="isLoading"
                        @click="model = false"
                    />
                </template>
            </v-card-item>

            <v-divider />

            <v-form v-model="formValid" class="pa-6" @submit.prevent>
                <v-text-field
                    v-model="name"
                    class="pt-4"
                    variant="outlined"
                    :rules="rules"
                    label="Access Name"
                    :counter="maxLength"
                    :maxlength="maxLength"
                    persistent-counter
                    :hide-details="false"
                    autofocus
                />
            </v-form>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn variant="outlined" color="default" block :disabled="isLoading" @click="model = false">
                            Cancel
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn color="primary" variant="flat" block :loading="isLoading" :disabled="!formValid" @click="onRenameClick">
                            Save
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import {
    VDialog,
    VCard,
    VCardItem,
    VCardTitle,
    VDivider,
    VCardActions,
    VRow,
    VCol,
    VBtn,
    VForm,
    VTextField,
    VSheet,
} from 'vuetify/components';
import { Pencil, X } from 'lucide-vue-next';

import { useLoading } from '@/composables/useLoading';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/composables/useNotify';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { ValidationRule , MaxNameLengthRule, RequiredRule } from '@/types/common';
import { AccessGrant } from '@/types/accessGrants';
import { useConfigStore } from '@/store/modules/configStore';

const props = defineProps<{
    access: AccessGrant;
}>();

const emit = defineEmits<{
    'renamed': [];
}>();

const model = defineModel<boolean>({ required: true });

const agStore = useAccessGrantsStore();
const configStore = useConfigStore();
const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const formValid = ref<boolean>(false);
const name = ref<string>('');

/**
 * Returns the maximum input length.
 */
const maxLength = computed<number>(() => {
    return configStore.state.config.maxNameCharacters;
});

/**
 * Returns an array of validation rules applied to the input.
 */
const rules = computed<ValidationRule<string>[]>(() => {
    return [
        RequiredRule,
        MaxNameLengthRule,
    ];
});

/**
 * Renames the access grant.
 */
async function onRenameClick(): Promise<void> {
    if (!formValid.value) return;
    await withLoading(async () => {
        try {
            await agStore.updateAccessGrant(props.access.id, name.value);
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.RENAME_ACCESS_GRANT_DIALOG);
            return;
        }

        notify.success('Access grant renamed successfully.');
        emit('renamed');
        model.value = false;
    });
}

watch(() => model.value, shown => {
    if (!shown) return;
    name.value = props.access.name;
}, { immediate: true });
</script>
