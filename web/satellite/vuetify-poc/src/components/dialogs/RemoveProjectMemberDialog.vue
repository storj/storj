// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="410px"
        transition="fade-transition"
        :persistent="isLoading"
    >
        <v-card rounded="xlg">
            <v-card-item class="pl-7 py-4">
                <template #prepend>
                    <v-sheet
                        class="bg-on-surface-variant d-flex justify-center align-center"
                        width="40"
                        height="40"
                        rounded="lg"
                    >
                        <img src="@poc/assets/icon-remove-member.svg" alt="member icon">
                    </v-sheet>
                </template>
                <v-card-title class="font-weight-bold">Remove member</v-card-title>
                <template #append>
                    <v-btn
                        icon="$close"
                        variant="text"
                        size="small"
                        color="default"
                        :disabled="isLoading"
                        @click="model = false"
                    />
                </template>
            </v-card-item>

            <v-card-item class="px-7 py-0">
                <v-divider />

                <p class="py-4">The following team members will be removed. This action cannot be undone.</p>

                <v-divider />
            </v-card-item>

            <v-card-item class="px-7 pt-4 pb-1">
                <v-chip
                    v-for="email in firstThreeSelected"
                    :key="email"
                    rounded
                    class="mb-3"
                >
                    <template #default>
                        <div class="max-width">
                            <p :title="email" class="text-truncate">{{ email }}</p>
                        </div>
                    </template>
                </v-chip>
                <v-chip v-if="props.emails.length > 3" rounded class="mb-3">
                    + {{ props.emails.length - 3 }} more
                </v-chip>
            </v-card-item>

            <v-card-item class="px-7 py-0">
                <v-alert variant="tonal" class="mb-4 pa-4" color="warning">
                    <template #text>
                        <strong>Please note:</strong> any access grants they have created will still provide
                        them with full access. If necessary, please revoke these access grants to ensure
                        the security of your data.
                    </template>
                </v-alert>

                <v-divider />
            </v-card-item>

            <v-card-actions class="pa-7">
                <v-row>
                    <v-col>
                        <v-btn variant="outlined" color="default" block :disabled="isLoading" @click="model = false">
                            Cancel
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn color="error" variant="flat" block :loading="isLoading" @click="onDelete">
                            Remove
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import {
    VAlert,
    VDialog,
    VCard,
    VCardItem,
    VSheet,
    VCardTitle,
    VDivider,
    VCardActions,
    VRow,
    VCol,
    VBtn,
    VChip,
} from 'vuetify/components';

import { useProjectsStore } from '@/store/modules/projectsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/utils/hooks';
import { useProjectMembersStore } from '@/store/modules/projectMembersStore';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

const props = defineProps<{
    modelValue: boolean,
    emails: string[],
}>();

const emit = defineEmits<{
    'update:modelValue': [value: boolean],
    'deleted': [];
}>();

const model = computed<boolean>({
    get: () => props.modelValue,
    set: value => emit('update:modelValue', value),
});

const analyticsStore = useAnalyticsStore();
const configStore = useConfigStore();
const projectsStore = useProjectsStore();
const pmStore = useProjectMembersStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const firstThreeSelected = computed<string[]>(() => props.emails.slice(0, 3));

async function onDelete(): Promise<void> {
    await withLoading(async () => {
        try {
            await pmStore.deleteProjectMembers(projectsStore.state.selectedProject.id, props.emails);
            notify.success('Members were successfully removed from the project');
            emit('deleted');
            model.value = false;
        } catch (error) {
            error.message = `Error removing project members. ${error.message}`;
            notify.notifyError(error, AnalyticsErrorEventSource.PROJECT_MEMBERS_HEADER);
        }
    });
}
</script>

<style scoped lang="scss">
.max-width {
    max-width: 320px;
}
</style>
