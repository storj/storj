// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog v-model="model" width="auto" transition="fade-transition">
        <v-card
            rounded="xlg"
            title="Undisqualify Node"
            subtitle="Remove disqualification from this storage node"
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
                        <v-textarea
                            v-model="reason"
                            :rules="[RequiredRule]"
                            label="Reason"
                            placeholder="Enter a reason for undisqualifying this node."
                            variant="solo-filled"
                            hide-details="auto"
                            autofocus
                            flat
                        />
                    </v-col>
                </v-row>

                <v-alert class="mt-6" title="Warning" variant="tonal" color="warning" rounded="lg">
                    This will clear the disqualification status of this node, allowing it to
                    receive data again.
                </v-alert>
            </div>

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn variant="outlined" color="default" block :disabled="isLoading" @click="model = false">Cancel</v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            color="primary" variant="flat"
                            :loading="isLoading"
                            :disabled="!reason"
                            block
                            @click="undisqualify"
                        >
                            Undisqualify Node
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { VAlert, VBtn, VCard, VCardActions, VCol, VDialog, VRow, VTextarea, VTextField } from 'vuetify/components';
import { X } from 'lucide-vue-next';
import { ref, watch } from 'vue';

import { useLoading } from '@/composables/useLoading';
import { useNodesStore } from '@/store/nodes';
import { useNotify } from '@/composables/useNotify';
import { RequiredRule } from '@/types/common';

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

function undisqualify() {
    withLoading(async () => {
        try {
            await nodesStore.undisqualifyNode(props.nodeId, reason.value);
            notify.success('Node undisqualified successfully');
            model.value = false;
            emit('success');
        } catch (e) {
            notify.error(e);
        }
    });
}

watch(model, (newVal) => {
    if (newVal) reason.value = '';
});
</script>
