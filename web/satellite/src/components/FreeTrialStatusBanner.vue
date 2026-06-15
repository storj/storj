// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-system-bar v-if="shouldShow" color="secondary" height="40">
        <div class="d-flex align-center justify-center w-100 ga-2">
            <span class="text-body-small font-weight-bold">Free trial remaining</span>
            <v-chip size="small">
                <template #prepend>
                    <calendar :size="14" class="mr-1" />
                </template>
                {{ daysLeft > 0 ? `${daysLeft} Day${daysLeft === 1 ? '' : 's'}` : 'Less than a day' }}
            </v-chip>
            <v-chip v-if="projectsStore.state.totalLimits.storageLimit" size="small">
                <template #prepend>
                    <cloud :size="14" class="mr-1" />
                </template>
                {{ storageRemaining }} remaining
            </v-chip>
            <v-btn
                color="primary"
                variant="flat"
                size="small"
                :append-icon="ArrowRight"
                @click="onUpgrade"
            >
                Upgrade Now
            </v-btn>
        </div>
    </v-system-bar>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue';
import { VBtn, VChip, VSystemBar } from 'vuetify/components';
import { ArrowRight, Calendar, Cloud } from '@lucide/vue';

import { useConfigStore } from '@/store/modules/configStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useAppStore } from '@/store/modules/appStore';
import { Dimensions, Size } from '@/utils/bytesSize';
import { Duration } from '@/utils/time';

const configStore = useConfigStore();
const usersStore = useUsersStore();
const projectsStore = useProjectsStore();
const appStore = useAppStore();

const shouldShow = computed<boolean>(() => {
    const expiration = usersStore.state.user.trialExpiration;
    if (!expiration || expiration.getTime() <= Date.now()) return false;
    if (!configStore.billingEnabled) return false;
    return !usersStore.state.user.isPaid;
});

const daysLeft = computed<number>(() => {
    const expiration = usersStore.state.user.trialExpiration;
    if (!expiration) return 0;
    const diffMs = expiration.getTime() - Date.now();
    if (diffMs <= 0) return 0;
    return new Duration(diffMs * 1_000_000).days;
});

const storageRemaining = computed<string>(() => {
    const limits = projectsStore.state.totalLimits;
    const limit = limits.storageLimit || usersStore.state.user.projectStorageLimit;
    const available = Math.max(0, limit - limits.storageUsed);
    const size = new Size(available, 2);
    if (size.label === Dimensions.Bytes) return '0 B';
    return `${size.formattedBytes.replace(/\.0+$/, '')} ${size.label}`;
});

onMounted(() => {
    if (!shouldShow.value) return;
    projectsStore.getTotalLimits().catch(() => { /* empty */});
});

function onUpgrade(): void {
    appStore.toggleUpgradeFlow(true);
}
</script>
