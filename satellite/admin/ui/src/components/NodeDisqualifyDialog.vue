// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog v-model="model" width="auto" transition="fade-transition">
        <v-card
            rounded="xlg"
            title="Disqualify Node"
            subtitle="Disqualify this storage node from the network"
        >
            <template #append>
                <v-btn
                    :icon="X" :disabled="isLoading"
                    variant="text" size="small" color="default" @click="model = false"
                />
            </template>

            <div class="pa-6">
                <v-row>
                    <v-col cols="12">
                        <v-text-field
                            :model-value="nodeId"
                            label="Node ID"
                            variant="solo-filled"
                            hide-details="auto"
                            flat readonly
                        />
                    </v-col>
                    <v-col cols="12">
                        <v-select
                            v-model="disqualificationReason"
                            :items="disqualificationReasonOptions"
                            :rules="[RequiredRule]"
                            label="Disqualification Reason"
                            variant="solo-filled"
                            hide-details="auto"
                            flat
                        />
                    </v-col>
                    <v-col cols="12">
                        <v-textarea
                            v-model="reason"
                            :rules="[RequiredRule]"
                            label="Reason"
                            placeholder="Enter a reason for disqualifying this node."
                            variant="solo-filled"
                            hide-details="auto"
                            flat
                        />
                    </v-col>
                </v-row>

                <v-alert class="mt-6" title="Warning" variant="tonal" color="error" rounded="lg">
                    This will disqualify the node, preventing it from receiving data.
                </v-alert>
            </div>

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn variant="outlined" color="default" block :disabled="isLoading" @click="model = false">Cancel</v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            color="error" variant="flat"
                            :loading="isLoading"
                            :disabled="!reason || !disqualificationReason"
                            block
                            @click="disqualify"
                        >
                            Disqualify Node
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { VAlert, VBtn, VCard, VCardActions, VCol, VDialog, VRow, VSelect, VTextarea, VTextField } from 'vuetify/components';
import { X } from 'lucide-vue-next';
import { ref, watch } from 'vue';

import { useLoading } from '@/composables/useLoading';
import { useNodesStore } from '@/store/nodes';
import { useNotify } from '@/composables/useNotify';
import { RequiredRule } from '@/types/common';

const disqualificationReasonOptions = [
    { title: 'Audit Failure', value: 'audit_failure' },
    { title: 'Suspension', value: 'suspension' },
    { title: 'Node Offline', value: 'node_offline' },
    { title: 'Unknown', value: 'unknown' },
];

const nodesStore = useNodesStore();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const model = defineModel<boolean>({ required: true });

const props = defineProps<{
    nodeId: string;
}>();

const emit = defineEmits<{
    (e: 'success'): void;
}>();

const reason = ref('');
const disqualificationReason = ref('unknown');

function disqualify() {
    withLoading(async () => {
        try {
            await nodesStore.disqualifyNode(props.nodeId, reason.value, disqualificationReason.value);
            notify.success('Node disqualified successfully');
            model.value = false;
            emit('success');
        } catch (e) {
            notify.error(e);
        }
    });
}

watch(model, (newVal) => {
    if (newVal) {
        reason.value = '';
        disqualificationReason.value = 'unknown';
    }
});
</script>
