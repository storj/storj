// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form class="pa-6" @submit.prevent>
        <v-row>
            <v-col>
                <p>Select the access configuration{{ app ? ` for ${app.name}` : '' }}.</p>
                <v-chip-group
                    v-model="flowType"
                    class="mt-2 mb-3"
                    selected-class="font-weight-bold"
                    variant="outlined"
                    mandatory
                    column
                    @update:model-value="val => emit('setFlowType', val)"
                >
                    <v-chip
                        :key="FlowType.FullAccess"
                        :value="FlowType.FullAccess"
                        filter
                    >
                        Full Access
                    </v-chip>
                    <v-chip
                        :key="FlowType.Advanced"
                        :value="FlowType.Advanced"
                        filter
                    >
                        Advanced
                    </v-chip>
                </v-chip-group>
                <v-alert v-if="flowType === FlowType.FullAccess" variant="tonal" width="auto">
                    <template v-if="app">
                        <p class="text-subtitle-2 font-weight-bold">Full Access</p>
                        <p class="text-subtitle-2">
                            The app will be provided full permissions access to all the buckets in this project. 1-click setup.
                        </p>
                        <p class="text-subtitle-2 font-weight-bold">Best for trying out an app.</p>
                    </template>
                    <p v-else class="text-subtitle-2">
                        The access key will have full permissions access to all the buckets and data in this project.
                    </p>
                </v-alert>
                <v-alert v-else variant="tonal" width="auto">
                    <template v-if="app">
                        <p class="text-subtitle-2 font-weight-bold">Advanced Setup</p>
                        <p class="text-subtitle-2">
                            You can choose what permissions to give this app, and which buckets it can access in this project.
                        </p>
                        <p class="text-subtitle-2 font-weight-bold">Select if you want more control of the access.</p>
                    </template>
                    <p v-else class="text-subtitle-2">
                        You can choose the permissions, select buckets, and set an expiry date for this access key.
                    </p>
                </v-alert>
            </v-col>
        </v-row>
    </v-form>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { VAlert, VChip, VChipGroup, VCol, VForm, VRow } from 'vuetify/components';

import { FlowType } from '@/types/setupAccess';
import { IDialogFlowStep } from '@/types/common';
import { Application } from '@/types/applications';

withDefaults(defineProps<{
    app?: Application
}>(), {
    app: undefined,
});

const emit = defineEmits<{
    'setFlowType': [flowType: FlowType]
}>();

const flowType = ref<FlowType>(FlowType.FullAccess);

defineExpose<IDialogFlowStep>({
    onExit: () => {
        emit('setFlowType', flowType.value);
    },
});
</script>
