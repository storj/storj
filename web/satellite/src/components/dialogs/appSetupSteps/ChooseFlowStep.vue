// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form class="pa-7" @submit.prevent>
        <v-row>
            <v-col>
                <p>Setup access to third-party applications.</p>
                <v-chip-group
                    v-model="flowType"
                    class="my-3"
                    selected-class="font-weight-bold"
                    color="primary"
                    mandatory
                    column
                    @update:modelValue="val => emit('setFlowType', val)"
                >
                    <v-chip
                        :key="FlowType.Default"
                        value="default"
                        color="green"
                        variant="outlined"
                        rounded
                        filter
                    >
                        Full Access
                    </v-chip>
                    <v-chip
                        :key="FlowType.Advanced"
                        value="advanced"
                        color="purple"
                        variant="outlined"
                        rounded
                        filter
                    >
                        Advanced
                    </v-chip>
                </v-chip-group>
                <v-alert v-if="flowType === FlowType.Default" variant="tonal" color="green" width="auto">
                    <p class="text-subtitle-2 font-weight-bold">Full Access</p>
                    <p class="text-subtitle-2">The app will be provided full permissions access to all the buckets in this project. 1-click setup.</p>
                    <p class="text-subtitle-2 font-weight-bold">Best for trying out an app.</p>
                </v-alert>
                <v-alert v-else variant="tonal" color="purple" width="auto">
                    <p class="text-subtitle-2 font-weight-bold">Advanced Setup</p>
                    <p class="text-subtitle-2">You can choose what permissions to give this app, and which buckets it can access in this project.</p>
                    <p class="text-subtitle-2 font-weight-bold">Select if you want more control of the access.</p>
                </v-alert>
            </v-col>
        </v-row>
    </v-form>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { VAlert, VChip, VChipGroup, VCol, VForm, VRow } from 'vuetify/components';

import { FlowType } from '@/types/createAccessGrant';
import { IDialogFlowStep } from '@/types/common';

const emit = defineEmits<{
    'setFlowType': [flowType: FlowType]
}>();

const flowType = ref<FlowType>(FlowType.Default);

defineExpose<IDialogFlowStep>({
    onExit: () => {
        emit('setFlowType', flowType.value);
    },
});
</script>
