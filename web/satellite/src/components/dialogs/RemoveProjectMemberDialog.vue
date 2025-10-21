// Copyright (C) 2023 Storj Labs, Inc.
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
                        <component :is="UserMinus" :size="18" />
                    </v-sheet>
                </template>
                <v-card-title class="font-weight-bold">Remove {{ memberInviteText }}</v-card-title>
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

            <v-card-item class="pa-6">
                <p class="mb-3">
                    The following {{ removables.filter(e => !e.isInvite).length ? "team" : "" }}
                    {{ memberInviteText }} will be removed.
                </p>

                <v-chip
                    v-for="removable in firstThreeSelected"
                    :key="removable.email"
                    class="mb-4 mr-1"
                >
                    <template #default>
                        <div class="max-width">
                            <p :title="removable.email" class="text-truncate">{{ removable.email }}</p>
                        </div>
                    </template>
                </v-chip>
                <v-chip v-if="removables.length > 3" rounded class="mb-3 mr-1">
                    + {{ removables.length - 3 }} more
                </v-chip>

                <v-alert variant="tonal" class="pa-4" color="warning">
                    <template #text>
                        <strong>Important:</strong> Any access keys created could still provide data access to removed members. If necessary, please revoke these access keys to ensure the security of your data.
                    </template>
                </v-alert>
            </v-card-item>

            <v-divider />

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
import { UserMinus, X } from 'lucide-vue-next';

import { useProjectsStore } from '@/store/modules/projectsStore';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { useProjectMembersStore } from '@/store/modules/projectMembersStore';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

const props = defineProps<{
    removables: { email:string, isInvite: boolean }[],
}>();

const model = defineModel<boolean>({ required: true });

const emit = defineEmits<{
    (event: 'deleted'): void;
}>();

const projectsStore = useProjectsStore();
const pmStore = useProjectMembersStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const firstThreeSelected = computed<{ email:string, isInvite: boolean }[]>(() => props.removables.slice(0, 3));

const memberInviteText = computed<string>(() => {
    const membersLength = props.removables.filter(e => !e.isInvite).length;
    const invitesLength = props.removables.length - membersLength;

    const memberTxt = `member${membersLength > 1 ? 's' : ''}`;
    const inviteTxt = `invite${invitesLength > 1 ? 's' : ''}`;
    if (membersLength && invitesLength) {
        return `${memberTxt} and ${inviteTxt}`;
    }
    if (invitesLength) {
        return inviteTxt;
    }
    return memberTxt;
});

async function onDelete(): Promise<void> {
    await withLoading(async () => {
        try {
            await pmStore.deleteProjectMembers(projectsStore.state.selectedProject.id, props.removables.map(e => e.email));
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
