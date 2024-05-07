// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="410px"
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
                        <img src="@/assets/icon-remove-member.svg" alt="member icon">
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

            <v-card-item class="px-6 py-0">
                <v-divider />

                <p class="py-4">The following team members will be removed.</p>

                <v-divider />
            </v-card-item>

            <v-card-item class="px-6 pt-4 pb-1">
                <v-chip
                    v-for="email in firstThreeSelected"
                    :key="email"
                    rounded
                    class="mb-3 mr-1"
                >
                    <template #default>
                        <div class="max-width">
                            <p :title="email" class="text-truncate">{{ email }}</p>
                        </div>
                    </template>
                </v-chip>
                <v-chip v-if="props.emails.length > 3" rounded class="mb-3 mr-1">
                    + {{ props.emails.length - 3 }} more
                </v-chip>
            </v-card-item>

            <v-card-item class="px-6 py-0">
                <v-alert variant="tonal" class="mb-4 pa-4" color="warning" border="start">
                    <template #text>
                        <strong>Important:</strong> Any access keys created could still provide data access to removed members. If necessary, please revoke these access keys to ensure the security of your data.
                    </template>
                </v-alert>

                <v-divider />
            </v-card-item>

            <v-card-actions class="pa-6">
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
    emails: string[],
}>();

const model = defineModel<boolean>({ required: true });

const emit = defineEmits<{
    (event: 'deleted'): void;
}>();

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
