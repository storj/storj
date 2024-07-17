// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        min-width="320px"
        max-width="400px"
        activator="parent"
        transition="fade-transition"
        scrollable
    >
        <v-card rounded="xlg">
            <v-sheet>
                <v-card-item class="pa-6">
                    <template #prepend>
                        <v-card-title class="font-weight-bold">
                            {{ currentTitle }}
                        </v-card-title>
                    </template>

                    <template #append>
                        <v-btn
                            icon="$close"
                            variant="text"
                            size="small"
                            color="default"
                            @click="model = false"
                        />
                    </template>
                </v-card-item>
            </v-sheet>

            <v-divider />

            <v-window v-model="step">
                <v-window-item :value="NewDomainFlowStep.CustomDomain">
                    <v-form class="pa-7 pb-3">
                        <v-row>
                            <v-col cols="12">
                                <p class="mb-3">Enter your domain name (URL)</p>
                                <v-text-field
                                    label="Website URL"
                                    placeholder="https://yourdomain.com"
                                    variant="outlined"
                                    color="default"
                                    hide-details
                                />
                            </v-col>

                            <v-col>
                                <p class="mb-3">Select a bucket to share files</p>
                                <v-select label="Bucket" />
                            </v-col>
                        </v-row>
                    </v-form>
                </v-window-item>

                <v-window-item :value="NewDomainFlowStep.SetupCNAME">
                    <v-form class="pa-3">
                        <v-card-text>
                            In your DNS provider, create CNAME record.
                            <v-text-field variant="solo-filled" flat class="my-4" density="comfortable" label="Hostname" model-value="www.example.com" readonly hide-details />
                            <v-text-field variant="solo-filled" flat class="my-4" density="comfortable" label="Content" model-value="link.storjshare.io." readonly hide-details />
                            <v-alert type="info" variant="tonal" class="mb-4">Ensure you include the dot . at the end</v-alert>
                            The next step is creating the TXT records.
                        </v-card-text>
                    </v-form>
                </v-window-item>

                <v-window-item :value="NewDomainFlowStep.SetupTXT">
                    <v-form class="pa-3">
                        <v-card-text>
                            In your DNS provider, create 3 TXT records.
                            <v-text-field variant="solo-filled" flat class="my-4" density="comfortable" label="TXT Hostname" model-value="txt-www.example.com" readonly hide-details />
                            <v-text-field variant="solo-filled" flat class="my-4" density="comfortable" label="TXT1 Content" model-value="storj-root:bucket/prefix" readonly hide-details />
                            <v-text-field variant="solo-filled" flat class="my-4" density="comfortable" label="TXT2 Content" model-value="storj-access:abcdefghijklmnopqrstuvwxzy" readonly hide-details />
                            <v-text-field variant="solo-filled" flat class="my-4" density="comfortable" label="TXT3 Content" model-value="storj-tls:true" readonly hide-details />
                        </v-card-text>
                    </v-form>
                </v-window-item>

                <v-window-item :value="NewDomainFlowStep.VerifyDomain">
                    <v-form class="pa-3">
                        <v-card-text>
                            Check to make sure your DNS records are ready.
                            <v-btn block variant="tonal" class="mt-3">Check DNS</v-btn>
                        </v-card-text>
                    </v-form>
                </v-window-item>

                <v-window-item :value="NewDomainFlowStep.DomainConnected">
                    <v-form class="pa-3">
                        <v-card-text>
                            <v-alert type="success" variant="tonal">
                                You should now be able to access your content using your custom domain.
                            </v-alert>
                            <v-alert type="info" variant="tonal" class="my-4">
                                DNS propagation usually takes less than a few hours, but can take up to 48 hours in some cases.
                            </v-alert>
                        </v-card-text>
                    </v-form>
                </v-window-item>
            </v-window>

            <v-divider />

            <v-card-actions class="pa-7">
                <v-row>
                    <v-col>
                        <v-btn
                            v-if="step === NewDomainFlowStep.CustomDomain"
                            variant="outlined"
                            color="default"
                            href="https://docs.storj.io/dcs/code/static-site-hosting/custom-domains"
                            target="_blank"
                            rel="noopener noreferrer"
                            block
                        >
                            Learn More
                        </v-btn>
                        <v-btn
                            v-if="step > NewDomainFlowStep.CustomDomain"
                            variant="outlined"
                            color="default"
                            block
                            @click="step--"
                        >
                            Back
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            v-if="step < NewDomainFlowStep.DomainConnected"
                            color="primary"
                            variant="flat"
                            block
                            @click="step++"
                        >
                            Next
                        </v-btn>
                        <v-btn
                            v-if="step === NewDomainFlowStep.DomainConnected"
                            color="primary"
                            variant="flat"
                            block
                            @click="model = false"
                        >
                            Finish
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import {
    VAlert,
    VBtn,
    VCard,
    VCardTitle,
    VCardActions,
    VCardItem,
    VCardText,
    VCol,
    VDialog,
    VDivider,
    VForm,
    VRow,
    VSelect,
    VSheet,
    VTextField,
    VWindow,
    VWindowItem,
} from 'vuetify/components';

import { NewDomainFlowStep } from '@/types/domains';

const model = defineModel<boolean>({ required: true });

const step = ref<NewDomainFlowStep>(NewDomainFlowStep.CustomDomain);

const currentTitle = computed<string>(() => {
    switch (step.value) {
    case NewDomainFlowStep.CustomDomain: return 'Setup Custom Domain';
    case NewDomainFlowStep.SetupCNAME: return 'Setup CNAME';
    case NewDomainFlowStep.SetupTXT: return 'Setup TXT';
    case NewDomainFlowStep.VerifyDomain: return 'Verify Domain';
    default: return 'Domain Connected';
    }
});
</script>
