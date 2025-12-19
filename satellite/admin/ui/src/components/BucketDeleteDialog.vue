// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog v-model="dialog" activator="parent" width="auto" transition="fade-transition">
        <v-card rounded="xlg">
            <v-sheet>
                <v-card-item class="pl-7 py-4">
                    <template #prepend>
                        <v-card-title class="font-weight-bold">
                            Delete Bucket
                        </v-card-title>
                    </template>

                    <template #append>
                        <v-btn icon="$close" variant="text" size="small" color="default" @click="dialog = false" />
                    </template>
                </v-card-item>
            </v-sheet>

            <v-divider />

            <v-form class="pa-7">
                <v-row>
                    <v-col cols="12">
                        <p>Please enter the reason for deleting this bucket.</p>
                    </v-col>
                </v-row>
                <v-row>
                    <v-col cols="12">
                        <v-select
                            label="Deleting reason" placeholder="Select one or more reasons"
                            :items="['Reason 1', 'Reason 2', 'Reason 3', 'Other']" multiple variant="outlined" autofocus required
                            hide-details="auto"
                        />
                    </v-col>
                </v-row>

                <v-row>
                    <v-col cols="12">
                        <v-text-field
                            model-value="Backups" label="Bucket Name" variant="solo-filled" flat readonly
                            hide-details="auto"
                        />
                    </v-col>
                    <v-col cols="12">
                        <v-text-field
                            model-value="itacker@gmail.com" label="Account Email" variant="solo-filled" flat readonly
                            hide-details="auto"
                        />
                    </v-col>
                </v-row>

                <v-row>
                    <v-col>
                        <v-alert variant="tonal" color="error" rounded="lg">This will delete the bucket and all it's data.</v-alert>
                    </v-col>
                </v-row>
            </v-form>

            <v-divider />

            <v-card-actions class="pa-7">
                <v-row>
                    <v-col>
                        <v-btn variant="outlined" color="default" block @click="dialog = false">Cancel</v-btn>
                    </v-col>
                    <v-col>
                        <v-btn color="error" variant="flat" block @click="onButtonClick">Delete Bucket</v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>

    <v-snackbar v-model="snackbar" :timeout="7000" color="success">
        The bucket was deleted successfully.
        <template #actions>
            <v-btn color="default" variant="text" @click="snackbar = false">
                Close
            </v-btn>
        </template>
    </v-snackbar>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import {
    VDialog,
    VCard,
    VSheet,
    VCardItem,
    VCardTitle,
    VBtn,
    VDivider,
    VForm,
    VRow,
    VCol,
    VSelect,
    VTextField,
    VCardActions,
    VSnackbar,
    VAlert,
} from 'vuetify/components';

const snackbar = ref<boolean>(false);
const dialog = ref<boolean>(false);

function onButtonClick() {
    snackbar.value = true;
    dialog.value = false;
}
</script>