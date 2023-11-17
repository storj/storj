// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="pa-7">
        <v-row>
            <v-col cols="12">
                Copy or save the S3 credentials as they will only appear once.
            </v-col>
            <save-buttons :items="saveItems" :name="name" type="S3-credentials" />
            <v-divider class="my-3" />

            <v-col cols="12">
                <text-output-area label="Access Key" :value="accessKey" :tooltip-disabled="isTooltipDisabled" show-copy />
            </v-col>
            <v-col cols="12">
                <text-output-area label="Secret Key" :value="secretKey" :tooltip-disabled="isTooltipDisabled" show-copy />
            </v-col>
            <v-col cols="12">
                <text-output-area label="Endpoint" :value="endpoint" :tooltip-disabled="isTooltipDisabled" show-copy />
            </v-col>
        </v-row>
    </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import { VRow, VCol, VDivider } from 'vuetify/components';

import { SaveButtonsItem } from '@poc/types/common';

import TextOutputArea from '@poc/components/dialogs/createAccessSteps/TextOutputArea.vue';
import SaveButtons from '@poc/components/dialogs/commonPassphraseSteps/SaveButtons.vue';

const props = defineProps<{
    name: string;
    accessKey: string;
    secretKey: string;
    endpoint: string;
}>();

const isTooltipDisabled = ref<boolean>(false);

const saveItems = computed<SaveButtonsItem[]>(() => [
    { name: 'Access Key', value: props.accessKey },
    { name: 'Secret Key', value: props.secretKey },
    { name: 'Endpoint', value: props.endpoint },
]);

defineExpose({
    title: 'S3 Credentials Generated',
    onEnter: () => isTooltipDisabled.value = false,
    onExit: () => isTooltipDisabled.value = true,
});
</script>
