// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form class="pa-6">
        <v-row>
            <v-col>
                <p>Save the access keys {{ app ? `for ${app.name}` : '' }} as they will only appear once.</p>
                <v-row class="mt-2">
                    <save-buttons :items="saveItems" :name="name" :type="accessType" />
                </v-row>
            </v-col>

            <template v-if="accessType === AccessType.APIKey">
                <v-col cols="12">
                    <text-output-area
                        label="Satellite Address"
                        :value="satelliteAddress"
                        show-copy
                    />
                </v-col>
                <v-col cols="12">
                    <text-output-area
                        label="API Key"
                        :value="cliAccess"
                        show-copy
                    />
                </v-col>
            </template>

            <v-col v-else-if="accessType === AccessType.AccessGrant" cols="12">
                <text-output-area
                    label="Access Grant"
                    :value="accessGrant"
                    show-copy
                />
            </v-col>

            <template v-else>
                <v-col v-if="credentials.freeTierRestrictedExpiration" cols="12">
                    <v-alert type="warning" variant="tonal">
                        These credentials will expire at {{ credentials.freeTierRestrictedExpiration.toLocaleString() }}.
                        <a class="text-decoration-underline text-cursor-pointer" @click="appStore.toggleUpgradeFlow(true)">Upgrade</a> your account to avoid expiration limits on future credentials.
                    </v-alert>
                </v-col>

                <v-col cols="12">
                    <text-output-area
                        label="Access Key"
                        :value="credentials.accessKeyId"
                        show-copy
                    />
                </v-col>
                <v-col cols="12">
                    <text-output-area
                        label="Secret Key"
                        :value="credentials.secretKey"
                        show-copy
                    />
                </v-col>
                <v-col cols="12">
                    <text-output-area
                        label="Endpoint"
                        :is-blurred="false"
                        :value="credentials.endpoint"
                        show-copy
                    />
                </v-col>
            </template>

            <v-col>
                <v-alert variant="tonal" color="info" class="mb-4">
                    <p class="text-subtitle-2">Tip: If you experience connection issues in some applications, try pasting the Endpoint URL without the "https://" prefix.</p>
                </v-alert>
                <v-alert variant="tonal" color="success">
                    <p class="text-subtitle-2 font-weight-bold">Next steps</p>
                    <p class="text-subtitle-2">Please read the documentation to find where to enter the access you created.</p>
                </v-alert>
            </v-col>
        </v-row>
    </v-form>
</template>

<script setup lang="ts">
import { VAlert, VCol, VForm, VRow } from 'vuetify/components';
import { computed } from 'vue';

import { EdgeCredentials } from '@/types/accessGrants';
import { AccessType } from '@/types/setupAccess';
import { useAppStore } from '@/store/modules/appStore';
import { useConfigStore } from '@/store/modules/configStore';
import { SaveButtonsItem } from '@/types/common';
import { Application } from '@/types/applications';

import SaveButtons from '@/components/dialogs/commonPassphraseSteps/SaveButtons.vue';
import TextOutputArea from '@/components/dialogs/accessSetupSteps/TextOutputArea.vue';

const props = withDefaults(defineProps<{
    name: string
    accessType: AccessType
    cliAccess: string
    accessGrant: string
    credentials: EdgeCredentials
    app?: Application
}>(), {
    app: undefined,
});

const appStore = useAppStore();
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
