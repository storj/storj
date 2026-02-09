// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="d-flex justify-space-between align-center my-4">
        <h3>Licenses</h3>
        <v-btn
            color="primary"
            :prepend-icon="Plus"
            @click="$emit('grant')"
        >
            Grant License
        </v-btn>
    </div>
    <v-card variant="flat" :border="true" rounded="xlg">
        <v-data-table
            :sort-by="sortBy"
            :headers="headers"
            :items="licenses"
            :search="search"
            :loading="isLoading"
            class="border-0"
            item-key="type"
            density="comfortable"
            hover
        >
            <template #top>
                <v-text-field
                    v-model="search"
                    label="Search"
                    :prepend-inner-icon="Search"
                    single-line
                    variant="solo-filled"
                    flat
                    hide-details
                    clearable
                    density="compact"
                    rounded="lg"
                    class="mx-2 mt-2 mb-2"
                />
            </template>

            <template #item.type="{ item }">
                <v-chip variant="tonal" color="primary" size="small" rounded="lg">
                    {{ item.type }}
                </v-chip>
            </template>

            <template #item.publicId="{ item }">
                <v-chip v-if="item.publicId" variant="tonal" size="small" rounded="lg" @click="goToProject(item.publicId)">
                    {{ item.publicId }}
                </v-chip>
                <span v-else class="text-disabled">All Projects</span>
            </template>

            <template #item.bucketName="{ item }">
                <v-chip v-if="item.bucketName" variant="tonal" size="small" rounded="lg">
                    {{ item.bucketName }}
                </v-chip>
                <span v-else class="text-disabled">All Buckets</span>
            </template>

            <template #item.key="{ item }">
                <v-chip v-if="item.key" color="success" variant="tonal" size="small" rounded="lg">
                    Yes
                </v-chip>
                <v-chip v-else variant="tonal" size="small" rounded="lg">
                    No
                </v-chip>
            </template>

            <template #item.expiresAt="{ item }">
                <span class="text-no-wrap">
                    {{ dateFns.format(item.expiresAt, 'fullDateTime') }}
                </span>
            </template>

            <template #item.revokedAt="{ item }">
                <v-chip v-if="item.revokedAt" color="error" variant="tonal" size="small" rounded="lg">
                    Revoked {{ dateFns.format(item.revokedAt, 'fullDateTime') }}
                </v-chip>
                <v-chip v-else-if="isExpired(item)" color="warning" variant="tonal" size="small" rounded="lg">
                    Expired
                </v-chip>
                <v-chip v-else color="success" variant="tonal" size="small" rounded="lg">
                    Active
                </v-chip>
            </template>

            <template #item.actions="{ item }">
                <v-btn
                    variant="outlined" color="default" size="small" class="text-caption" density="comfortable" icon
                    width="24" height="24"
                >
                    <LicenseActionsMenu
                        :license="item"
                        @revoke="license => $emit('revoke', license)"
                        @delete="license => $emit('delete', license)"
                    />
                    <v-icon :icon="MoreHorizontal" />
                </v-btn>
            </template>

            <template #no-data>
                <div class="text-center py-8">
                    <v-icon :icon="FileText" size="48" class="text-disabled mb-4" />
                    <p class="text-h6 text-disabled">No licenses found</p>
                    <p class="text-body-2 text-disabled mb-4">
                        This user doesn't have any licenses yet.
                    </p>
                    <v-btn
                        color="primary"
                        :prepend-icon="Plus"
                        @click="$emit('grant')"
                    >
                        Grant First License
                    </v-btn>
                </div>
            </template>
        </v-data-table>
    </v-card>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { VBtn, VCard, VTextField, VDataTable, VChip, VIcon } from 'vuetify/components';
import { Search, Plus, MoreHorizontal, FileText } from 'lucide-vue-next';
import { useDate } from 'vuetify';
import { useRouter } from 'vue-router';

import { DataTableHeader, SortItem } from '@/types/common';
import { UserLicense } from '@/api/client.gen';
import { useLoading } from '@/composables/useLoading';
import { useUsersStore } from '@/store/users';
import { ROUTES } from '@/router';

import LicenseActionsMenu from '@/components/LicenseActionsMenu.vue';

const props = defineProps<{
    userId: string;
}>();

defineEmits<{
    grant: [];
    revoke: [license: UserLicense];
    delete: [license: UserLicense];
}>();

const router = useRouter();
const dateFns = useDate();
const usersStore = useUsersStore();
const { isLoading, withLoading } = useLoading();

const search = ref<string>('');
const licenses = ref<UserLicense[]>([]);

const sortBy = ref<SortItem[]>([{ key: 'expiresAt', order: 'desc' }]);

const headers = computed<DataTableHeader[]>(() => [
    { title: 'Type', key: 'type', sortable: true },
    { title: 'Project', key: 'publicId', sortable: true },
    { title: 'Bucket', key: 'bucketName', sortable: true },
    { title: 'Key', key: 'key', sortable: true },
    { title: 'Expires At', key: 'expiresAt', sortable: true },
    { title: 'Status', key: 'revokedAt', sortable: true },
    { title: 'Actions', key: 'actions', sortable: false, align: 'end', width: '100' },
]);

function isExpired(license: UserLicense): boolean {
    if (!license.expiresAt) return false;
    return new Date(license.expiresAt) < new Date();
}

function goToProject(publicId: string) {
    router.push({
        name: ROUTES.ProjectDetail.name,
        query: { projectID: publicId },
    });
}

async function fetchLicenses() {
    await withLoading(async () => {
        licenses.value = await usersStore.getUserLicenses(props.userId);
    });
}

// Watch userId changes and fetch licenses
watch(() => props.userId, fetchLicenses, { immediate: true });

// Expose refresh method for parent component
defineExpose({
    refresh: fetchLicenses,
});
</script>
