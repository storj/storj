// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form class="pa-6">
        <v-row>
            <v-col>
                <p class="font-weight-bold mb-2">Access Created</p>
                <p>
                    Copy and save the access credentials {{ isAppSetup ? 'for this application' : '' }} as they will only appear once.
                </p>
                <v-row class="mt-2">
                    <save-buttons :items="saveItems" :name="name" :type="accessType" />
                </v-row>
            </v-col>

            <template v-if="accessType === AccessType.APIKey">
                <v-col cols="12">
                    <v-text-field
                        flat
                        density="comfortable"
                        label="Satellite Address"
                        :model-value="satelliteAddress"
                        readonly
                        hide-details
                    >
                        <template #append-inner>
                            <input-copy-button :value="satelliteAddress" />
                        </template>
                    </v-text-field>
                </v-col>
                <v-col cols="12">
                    <v-text-field
                        flat
                        density="comfortable"
                        label="API Key"
                        :model-value="cliAccess"
                        readonly
                        hide-details
                    >
                        <template #append-inner>
                            <input-copy-button :value="cliAccess" />
                        </template>
                    </v-text-field>
                </v-col>
            </template>

            <v-col v-else-if="accessType === AccessType.AccessGrant" cols="12">
                <v-text-field
                    flat
                    density="comfortable"
                    label="Access Grant"
                    :model-value="accessGrant"
                    readonly
                    hide-details
                >
                    <template #append-inner>
                        <input-copy-button :value="accessGrant" />
                    </template>
                </v-text-field>
            </v-col>

            <template v-else>
                <v-col cols="12">
                    <v-text-field
                        flat
                        density="comfortable"
                        label="Access Key"
                        :model-value="credentials.accessKeyId"
                        readonly
                        hide-details
                    >
                        <template #append-inner>
                            <input-copy-button :value="credentials.accessKeyId" />
                        </template>
                    </v-text-field>
                </v-col>
                <v-col cols="12">
                    <v-text-field
                        flat
                        density="comfortable"
                        label="Secret Key"
                        :model-value="credentials.secretKey"
                        readonly
                        hide-details
                    >
                        <template #append-inner>
                            <input-copy-button :value="credentials.secretKey" />
                        </template>
                    </v-text-field>
                </v-col>
                <v-col cols="12">
                    <v-text-field
                        flat
                        density="comfortable"
                        label="Endpoint"
                        :model-value="credentials.endpoint"
                        readonly
                        hide-details
                    >
                        <template #append-inner>
                            <input-copy-button :value="credentials.endpoint" />
                        </template>
                    </v-text-field>
                </v-col>
            </template>

            <v-col>
                <v-alert variant="tonal" color="info">
                    <p class="text-subtitle-2 font-weight-bold">Next steps</p>
                    <p class="text-subtitle-2">Please read the documentation to find where to enter the access you created.</p>
                </v-alert>
            </v-col>
        </v-row>
    </v-form>
</template>

<script setup lang="ts">
import { VAlert, VCol, VForm, VRow, VTextField } from 'vuetify/components';
import { computed } from 'vue';

import { EdgeCredentials } from '@/types/accessGrants';
import { AccessType } from '@/types/createAccessGrant';
import { useConfigStore } from '@/store/modules/configStore';
import { SaveButtonsItem } from '@/types/common';

import InputCopyButton from '@/components/InputCopyButton.vue';
import SaveButtons from '@/components/dialogs/commonPassphraseSteps/SaveButtons.vue';

const props = defineProps<{
    name: string
    isAppSetup: boolean
    accessType: AccessType
    cliAccess: string
    accessGrant: string
    credentials: EdgeCredentials
}>();

const configStore = useConfigStore();

const satelliteAddress = computed<string>(() => configStore.state.config.satelliteNodeURL);

/**
 * Returns items for save/download buttons based on access type.
 */
const saveItems = computed<SaveButtonsItem[]>(() => {
    if (props.accessType === AccessType.APIKey) {
        return [
            { name: 'Satellite Address', value: satelliteAddress.value },
            { name: 'API Key', value: props.cliAccess },
        ];
    }

    if (props.accessType === AccessType.AccessGrant) {
        return [props.accessGrant];
    }

    return [
        { name: 'Access Key', value: props.credentials.accessKeyId },
        { name: 'Secret Key', value: props.credentials.secretKey },
        { name: 'Endpoint', value: props.credentials.endpoint },
    ];
});
</script>
