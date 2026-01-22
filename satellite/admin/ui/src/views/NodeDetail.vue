// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <v-row dense>
            <v-col>
                <v-btn
                    variant="text"
                    :prepend-icon="ArrowLeft"
                    class="mb-4"
                    @click="goBack"
                >
                    Back
                </v-btn>
            </v-col>
        </v-row>
        <v-row>
            <v-col>
                <v-card rounded="xlg">
                    <v-card-title class="d-flex justify-space-between align-center pa-5 pb-3">
                        <div class="d-flex align-center">
                            <v-icon :icon="Server" class="mr-2" />
                            Node Details
                        </div>
                    </v-card-title>

                    <v-divider />

                    <v-card-text v-if="nodeInfo" class="pa-6">
                        <v-row dense>
                            <v-col cols="12" sm="6">
                                <div class="text-caption text-medium-emphasis mb-1">Node ID</div>
                                <div class="font-weight-medium text-body-2">{{ nodeInfo.id }}</div>
                            </v-col>
                            <v-col cols="12" sm="6">
                                <div class="text-caption text-medium-emphasis mb-1">Email</div>
                                <div class="font-weight-medium">{{ nodeInfo.email }}</div>
                            </v-col>
                        </v-row>

                        <v-divider class="my-4" />

                        <v-row dense>
                            <v-col cols="12" sm="6">
                                <div class="text-caption text-medium-emphasis mb-1">Address</div>
                                <div class="font-weight-medium">{{ nodeInfo.address }}</div>
                            </v-col>
                            <v-col cols="12" sm="6">
                                <div class="text-caption text-medium-emphasis mb-1">Country</div>
                                <div class="font-weight-medium">{{ nodeInfo.countryCode || 'Unknown' }}</div>
                            </v-col>
                        </v-row>

                        <v-divider class="my-4" />

                        <v-row dense>
                            <v-col cols="12" sm="6">
                                <div class="text-caption text-medium-emphasis mb-1">Version</div>
                                <div class="font-weight-medium">{{ nodeInfo.version || 'Unknown' }}</div>
                            </v-col>
                            <v-col cols="12" sm="6">
                                <div class="text-caption text-medium-emphasis mb-1">Created</div>
                                <div class="font-weight-medium">{{ dateFns.format(nodeInfo.createdAt, 'fullDateTime') }}</div>
                            </v-col>
                        </v-row>

                        <v-divider class="my-4" />

                        <v-row dense>
                            <v-col cols="12" sm="6">
                                <div class="text-caption text-medium-emphasis mb-1">Last Contact Success</div>
                                <div class="font-weight-medium">{{ dateFns.format(nodeInfo.lastContactSuccess, 'fullDateTime') }}</div>
                            </v-col>
                            <v-col cols="12" sm="6">
                                <div class="text-caption text-medium-emphasis mb-1">Last Contact Failure</div>
                                <div class="font-weight-medium">
                                    {{ nodeInfo.lastContactFailure ? dateFns.format(nodeInfo.lastContactFailure, 'fullDateTime') : '—' }}
                                </div>
                            </v-col>
                        </v-row>

                        <v-divider class="my-4" />

                        <v-row dense>
                            <v-col cols="12" sm="6">
                                <div class="text-caption text-medium-emphasis mb-1">Free Disk</div>
                                <div class="font-weight-medium">{{ formatBytes(nodeInfo.freeDisk) }}</div>
                            </v-col>
                            <v-col cols="12" sm="6">
                                <div class="text-caption text-medium-emphasis mb-1">Piece Count</div>
                                <div class="font-weight-medium">{{ nodeInfo.pieceCount.toLocaleString() }}</div>
                            </v-col>
                        </v-row>

                        <v-divider class="my-4" />

                        <v-row dense>
                            <v-col cols="12" sm="6">
                                <div class="text-caption text-medium-emphasis mb-1">Vetted</div>
                                <v-chip
                                    v-if="nodeInfo.vettedAt"
                                    variant="tonal"
                                    color="success"
                                    size="small"
                                >
                                    {{ dateFns.format(nodeInfo.vettedAt, 'fullDate') }}
                                </v-chip>
                                <span v-else class="font-weight-medium">Not vetted</span>
                            </v-col>
                            <v-col cols="12" sm="6">
                                <div class="text-caption text-medium-emphasis mb-1">Disqualified</div>
                                <template v-if="nodeInfo.disqualified">
                                    <v-chip
                                        variant="tonal"
                                        color="error"
                                        size="small"
                                    >
                                        {{ dateFns.format(nodeInfo.disqualified, 'fullDate') }}
                                    </v-chip>
                                    <div v-if="nodeInfo.disqualificationReason" class="text-caption mt-1">
                                        Reason: {{ nodeInfo.disqualificationReason }}
                                    </div>
                                </template>
                                <span v-else class="font-weight-medium">—</span>
                            </v-col>
                        </v-row>

                        <v-divider class="my-4" />

                        <v-row dense>
                            <v-col cols="12" sm="6">
                                <div class="text-caption text-medium-emphasis mb-1">Wallet</div>
                                <div class="font-weight-medium text-truncate">{{ nodeInfo.wallet || '—' }}</div>
                            </v-col>
                            <v-col cols="12" sm="6">
                                <div class="text-caption text-medium-emphasis mb-1">Wallet Features</div>
                                <div v-if="nodeInfo.walletFeatures && nodeInfo.walletFeatures.length > 0">
                                    <v-chip
                                        v-for="feature in nodeInfo.walletFeatures"
                                        :key="feature"
                                        variant="tonal"
                                        size="small"
                                        class="mr-1 mb-1"
                                    >
                                        {{ feature }}
                                    </v-chip>
                                </div>
                                <span v-else class="font-weight-medium">—</span>
                            </v-col>
                        </v-row>

                        <template v-if="nodeInfo.exitInitiatedAt">
                            <v-divider class="my-4" />

                            <v-row dense>
                                <v-col cols="12">
                                    <div class="text-caption text-medium-emphasis mb-1">Graceful Exit</div>
                                    <div class="d-flex flex-wrap ga-2">
                                        <v-chip variant="tonal" color="warning" size="small">
                                            Initiated: {{ dateFns.format(nodeInfo.exitInitiatedAt, 'fullDate') }}
                                        </v-chip>
                                        <v-chip
                                            v-if="nodeInfo.exitFinishedAt"
                                            variant="tonal"
                                            :color="nodeInfo.exitSuccess ? 'success' : 'error'"
                                            size="small"
                                        >
                                            {{ nodeInfo.exitSuccess ? 'Completed' : 'Failed' }}: {{ dateFns.format(nodeInfo.exitFinishedAt, 'fullDate') }}
                                        </v-chip>
                                    </div>
                                </v-col>
                            </v-row>
                        </template>
                    </v-card-text>
                </v-card>
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { VBtn, VCard, VCardText, VCardTitle, VChip, VCol, VContainer, VDivider, VIcon, VRow } from 'vuetify/components';
import { ArrowLeft, Server } from 'lucide-vue-next';
import { useDate } from 'vuetify';

import { NodeFullInfo } from '@/api/client.gen';
import { useNotify } from '@/composables/useNotify';
import { useNodesStore } from '@/store/nodes';
import { Size } from '@/utils/bytesSize';
import { useAppStore } from '@/store/app';
import { ROUTES } from '@/router';

const appStore = useAppStore();
const nodesStore = useNodesStore();

const route = useRoute();
const router = useRouter();
const dateFns = useDate();
const notify = useNotify();

const nodeInfo = ref<NodeFullInfo | null>(null);

function formatBytes(bytes: number): string {
    const size = new Size(bytes);
    return `${size.formattedBytes} ${size.label}`;
}

function goBack(): void {
    if (window.history.length <= 1) router.push({ name: ROUTES.Accounts.name });
    else router.back();
}

function fetchNodeInfo() {
    const nodeID = route.params.nodeID as string;
    if (!nodeID) {
        goBack();
        return;
    }

    appStore.load(async () => {
        try {
            nodeInfo.value = await nodesStore.getNodeById(nodeID);
        } catch (e) {
            notify.error(e);
        }
    });
}

onMounted(() => fetchNodeInfo());
</script>
