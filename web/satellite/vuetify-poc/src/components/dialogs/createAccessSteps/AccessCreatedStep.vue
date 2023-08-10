// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="pa-8">
        <v-row>
            <v-col cols="12">
                Copy or save the Access Grant as it will only appear once.
            </v-col>
            <save-buttons :items="[ accessGrant ]" :access-name="name" file-name-base="access" />
            <v-divider class="my-3" />

            <v-col cols="12">
                <text-output-area ref="output" label="Access Grant" :value="accessGrant" :tooltip-disabled="isTooltipDisabled" />
            </v-col>
        </v-row>
    </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { VRow, VCol, VDivider } from 'vuetify/components';

import { CreateAccessStepComponent } from '@poc/types/createAccessGrant';

import TextOutputArea from '@poc/components/dialogs/createAccessSteps/TextOutputArea.vue';
import SaveButtons from '@poc/components/dialogs/createAccessSteps/SaveButtons.vue';

const props = defineProps<{
    name: string;
    accessGrant: string;
}>();

const output = ref<InstanceType<typeof TextOutputArea> | null>(null);
const isTooltipDisabled = ref<boolean>(false);

defineExpose<CreateAccessStepComponent>({
    title: 'Access Created',
    onEnter: () => isTooltipDisabled.value = false,
    onExit: () => isTooltipDisabled.value = true,
});
</script>
