// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="pa-7">
        <v-row>
            <v-col cols="12">
                Copy or save the satellite address and API key as they will only appear once.
            </v-col>
            <save-buttons :items="saveItems" :name="name" type="CLI-access" />
            <v-divider class="my-3" />

            <v-col cols="12">
                <text-output-area
                    label="Satellite Address"
                    :value="satelliteAddress"
                    :tooltip-disabled="isTooltipDisabled"
                    show-copy
                />
            </v-col>
            <v-col cols="12">
                <text-output-area
                    label="API Key"
                    :value="apiKey"
                    :tooltip-disabled="isTooltipDisabled"
                    show-copy
                />
            </v-col>
        </v-row>
    </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import { VRow, VCol, VDivider } from 'vuetify/components';

import { useConfigStore } from '@/store/modules/configStore';
import { SaveButtonsItem, DialogStepComponent } from '@poc/types/common';

import TextOutputArea from '@poc/components/dialogs/createAccessSteps/TextOutputArea.vue';
import SaveButtons from '@poc/components/dialogs/commonPassphraseSteps/SaveButtons.vue';

const props = defineProps<{
    name: string;
    apiKey: string;
}>();

const configStore = useConfigStore();

const isTooltipDisabled = ref<boolean>(false);

const satelliteAddress = computed<string>(() => configStore.state.config.satelliteNodeURL);

const saveItems = computed<SaveButtonsItem[]>(() => [
    { name: 'Satellite Address', value: satelliteAddress.value },
    { name: 'API Key', value: props.apiKey },
]);

defineExpose<DialogStepComponent>({
    title: 'CLI Access Created',
    onEnter: () => isTooltipDisabled.value = false,
    onExit: () => isTooltipDisabled.value = true,
});
</script>
