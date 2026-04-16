// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card-item class="pa-6">
        <template #prepend>
            <v-sheet
                class="border-sm d-flex justify-center align-center"
                width="40"
                height="40"
                rounded="lg"
            >
                <component :is="showLimitIncreaseForm ? Gauge : Box" :size="18" />
            </v-sheet>
        </template>

        <v-card-title class="font-weight-bold">{{ title }}</v-card-title>

        <template #append>
            <v-btn
                :icon="X"
                variant="text"
                size="small"
                color="default"
                :disabled="isLoading"
                @click="emit('cancel')"
            />
        </template>
    </v-card-item>

    <v-divider />

    <v-form v-if="user.hasPaidPrivileges" v-model="formValid" class="pa-6" @submit.prevent>
        <v-row>
            <template v-if="!showLimitIncreaseForm">
                <v-col cols="12">
                    You've reached your project limit. Request an increase to create more projects.
                </v-col>
            </template>
            <template v-else>
                <v-col cols="12">
                    Request a projects limit increase for your account.
                </v-col>
                <v-col cols="6">
                    <p>Projects Limit</p>
                    <v-text-field
                        class="edit-project-limit__text-field"
                        variant="solo-filled"
                        density="compact"
                        flat
                        readonly
                        :model-value="user.projectLimit"
                    />
                </v-col>
                <v-col cols="6">
                    <p>Requested Limit</p>
                    <v-text-field
                        class="edit-project-limit__text-field"
                        density="compact"
                        flat
                        type="number"
                        :rules="projectLimitRules"
                        :model-value="requestedLimit"
                        maxlength="4"
                        @update:model-value="v => requestedLimit = v"
                    />
                </v-col>
            </template>
        </v-row>
    </v-form>

    <v-row v-else class="pa-6">
        <v-col>
            Upgrade to Pro Account to create more projects and gain access to higher limits.
        </v-col>
    </v-row>

    <v-divider />

    <v-card-actions class="pa-6">
        <v-row>
            <v-col>
                <v-btn
                    variant="outlined"
                    color="default"
                    block
                    :disabled="isLoading"
                    @click="onBackOrCancel"
                >
                    {{ showLimitIncreaseForm ? 'Back' : 'Cancel' }}
                </v-btn>
            </v-col>
            <v-col>
                <v-btn
                    color="primary"
                    variant="flat"
                    :loading="isLoading"
                    block
                    :append-icon="buttonShowsArrow ? ArrowRight : ''"
                    @click="onPrimaryClick"
                >
                    {{ buttonTitle }}
                </v-btn>
            </v-col>
        </v-row>
    </v-card-actions>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import {
    VBtn,
    VCardActions,
    VCardItem,
    VCardTitle,
    VCol,
    VDivider,
    VForm,
    VRow,
    VSheet,
    VTextField,
} from 'vuetify/components';
import { ArrowRight, Box, Gauge, X } from 'lucide-vue-next';

import { RequiredRule, ValidationRule } from '@/types/common';
import { useLoading } from '@/composables/useLoading';
import { useUsersStore } from '@/store/modules/usersStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

const emit = defineEmits<{
    cancel: [];
    'show-upgrade': [];
    'update:loading': [value: boolean];
}>();

const usersStore = useUsersStore();
const configStore = useConfigStore();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const formValid = ref(false);
const showLimitIncreaseForm = ref(false);
const requestedLimit = ref(String(usersStore.state.user.projectLimit + 1));

const user = computed(() => usersStore.state.user);
const isLimitIncreaseRequestEnabled = computed(() => configStore.state.config.limitIncreaseRequestEnabled);

const title = computed(() => {
    if (user.value.hasPaidPrivileges && showLimitIncreaseForm.value) return 'Projects Limit Request';
    return 'Get More Projects';
});

const buttonTitle = computed(() => {
    if (!user.value.isPaid) return 'Upgrade';
    if (showLimitIncreaseForm.value) return 'Submit';
    return 'Request';
});

const buttonShowsArrow = computed(() => !user.value.isPaid || !isLimitIncreaseRequestEnabled.value);

const projectLimitRules = computed<ValidationRule<string>[]>(() => [
    RequiredRule,
    v => !(isNaN(+v) || !Number.isInteger(parseFloat(v))) || 'Invalid number',
    v => parseFloat(v) > 0 || 'Number must be positive',
]);

async function onPrimaryClick(): Promise<void> {
    if (!user.value.isPaid) {
        emit('show-upgrade');
        return;
    }

    if (!isLimitIncreaseRequestEnabled.value) {
        emit('cancel');
        window.open(`${configStore.supportUrl}?ticket_form_id=360000683212`, '_blank', 'noopener');
        return;
    }

    if (!showLimitIncreaseForm.value) {
        showLimitIncreaseForm.value = true;
        return;
    }

    if (!formValid.value) return;

    await withLoading(async () => {
        try {
            await usersStore.requestProjectLimitIncrease(requestedLimit.value);
        } catch (error) {
            error.message = `Failed to request project limit increase. ${error.message}`;
            notify.notifyError(error, AnalyticsErrorEventSource.CREATE_PROJECT_MODAL);
            return;
        }
        emit('cancel');
        notify.success('Project limit increase requested');
    });
}

function onBackOrCancel(): void {
    if (showLimitIncreaseForm.value) {
        showLimitIncreaseForm.value = false;
    } else {
        emit('cancel');
    }
}

function reset(): void {
    showLimitIncreaseForm.value = false;
    requestedLimit.value = String(usersStore.state.user.projectLimit + 1);
}

defineExpose({ reset });

watch(isLoading, v => emit('update:loading', v));
</script>
