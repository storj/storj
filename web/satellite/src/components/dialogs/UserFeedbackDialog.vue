// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        scrollable
        max-width="400px"
        transition="fade-transition"
        :persistent="isLoading"
    >
        <v-card>
            <v-card-item class="pa-6">
                <v-card-title class="font-weight-bold">Give Feedback</v-card-title>
                <template #append>
                    <v-btn
                        :icon="X"
                        variant="text"
                        size="small"
                        color="default"
                        @click="model = false"
                    />
                </template>
            </v-card-item>

            <v-divider />

            <v-card-item class="px-6 pb-0">
                <v-alert
                    border
                    variant="outlined"
                    color="info"
                >
                    <p class="text-body-2">
                        <strong>Do not use this form for account, billing, or support issues.</strong>
                        If you need help with those, please <a :href="supportURL" target="_blank" rel="noopener noreferrer">create a support ticket here.</a>
                    </p>
                </v-alert>
            </v-card-item>

            <v-card-item class="px-6">
                <v-form ref="form" v-model="formValid" class="pt-2" @submit.prevent="sendFeedback">
                    <v-select
                        v-model="type"
                        label="Type"
                        :items="Object.values(FeedbackType)"
                    />

                    <v-textarea
                        v-model="message"
                        class="mt-2"
                        variant="outlined"
                        :rules="[RequiredRule]"
                        :label="type === FeedbackType.Report ? 'Provide reproduction steps' : 'Write what you think'"
                        :placeholder="type === FeedbackType.Report ? 'Enter reproduction steps here' : 'Enter your feedback here'"
                        :maxlength="500"
                        required
                    />

                    <v-checkbox
                        v-model="allowContact"
                        class="no-min-height"
                        label="You may contact me for more details."
                    />
                </v-form>
            </v-card-item>
            <v-divider />
            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn
                            variant="outlined"
                            color="default"
                            block
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
                            :disabled="!formValid"
                            :loading="isLoading"
                            @click="sendFeedback"
                        >
                            Send
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import {
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
    VCardTitle,
    VCol,
    VDialog,
    VDivider,
    VForm,
    VRow,
    VSelect,
    VTextarea,
    VCheckbox,
    VAlert,
} from 'vuetify/components';
import { X } from 'lucide-vue-next';

import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { RequiredRule } from '@/types/common';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useConfigStore } from '@/store/modules/configStore';

const analyticsStore = useAnalyticsStore();
const configStore = useConfigStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

enum FeedbackType {
    General = 'General Feedback',
    Improvement = 'Improvement Idea',
    Report = 'Report an issue or bug',
}

const model = defineModel<boolean>({ required: true });

const type = ref<FeedbackType>(FeedbackType.General);
const message = ref<string>('');
const allowContact = ref<boolean>(false);
const formValid = ref(false);

const form = ref<VForm>();

const supportURL = computed<string>(() => configStore.state.config.generalRequestURL);

function sendFeedback(): void {
    if (!formValid.value) {
        return;
    }
    withLoading(async () => {
        try {
            await analyticsStore.sendUserFeedback({
                type: type.value,
                message: message.value,
                allowContact: allowContact.value,
            });
            notify.success('Feedback sent successfully');
            model.value = false;
        } catch (error) {
            notify.notifyError(error);
        }
    });
}

watch(model, val => {
    if (!val) {
        form.value?.reset();
        type.value = FeedbackType.General;
        message.value = '';
    }
});
</script>

<style scoped lang="scss">
.no-min-height :deep(.v-selection-control) {
    min-height: unset;
}
</style>
