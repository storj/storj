// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-snackbar
        vertical
        :timeout="4000"
        color="default"
        elevation="24"
        rounded="lg"
        class="upload-snackbar"
    >
        <v-row>
            <v-col>
                <v-expansion-panels theme="dark">
                    <v-expansion-panel
                        color="default"
                        rounded="lg"
                    >
                        <v-expansion-panel-title color="">
                            <span>Uploading 3 items</span>
                        </v-expansion-panel-title>
                        <v-progress-linear
                            rounded
                            model-value="73"
                            height="6"
                            color="success"
                            class="mt-1"
                        />
                        <v-expansion-panel-text>
                            <v-row class="pt-2">
                                <v-col cols="10">
                                    <p class="text-medium-emphasis">4 seconds left...</p>
                                </v-col>
                                <v-col cols="2">
                                    <v-tooltip text="Cancel all uploads">
                                        <template #activator="{ props: activatorProps }">
                                            <v-icon
                                                v-bind="activatorProps"
                                                icon="mdi-close-circle"
                                                @click="onCancel"
                                            />
                                        </template>
                                    </v-tooltip>
                                </v-col>
                            </v-row>
                        </v-expansion-panel-text>
                        <v-divider />
                        <v-expansion-panel-text>
                            <v-row class="pt-2">
                                <v-col cols="10">
                                    <p>Image.jpg</p>
                                </v-col>
                                <v-col cols="2">
                                    <v-tooltip text="Uploading...">
                                        <template #activator="{ props: activatorProps }">
                                            <v-progress-circular
                                                v-bind="activatorProps"
                                                :size="20"
                                                color="secondary"
                                                model-value="20"
                                            />
                                        </template>
                                    </v-tooltip>
                                </v-col>
                            </v-row>
                        </v-expansion-panel-text>
                        <v-divider />
                        <v-expansion-panel-text>
                            <v-row class="pt-2">
                                <v-col cols="10">
                                    <p>Video.mp4</p>
                                </v-col>
                                <v-col cols="2">
                                    <v-tooltip text="Upload complete">
                                        <template #activator="{ props: activatorProps }">
                                            <v-icon
                                                v-bind="activatorProps"
                                                icon="mdi-checkbox-marked-circle"
                                                color="success"
                                            />
                                        </template>
                                    </v-tooltip>
                                </v-col>
                            </v-row>
                        </v-expansion-panel-text>
                        <v-divider />
                        <v-expansion-panel-text>
                            <v-row class="pt-2">
                                <v-col cols="10">
                                    <p>Text.pdf</p>
                                </v-col>
                                <v-col cols="2">
                                    <v-tooltip text="Upload failed">
                                        <template #activator="{ props: activatorProps }">
                                            <v-icon
                                                v-bind="activatorProps"
                                                icon="mdi-information"
                                                color="warning"
                                            />
                                        </template>
                                    </v-tooltip>
                                </v-col>
                            </v-row>
                        </v-expansion-panel-text>
                        <v-divider />
                        <v-expansion-panel-text>
                            <v-row class="pt-2">
                                <v-col cols="10">
                                    <p>Bigvideo.mov</p>
                                </v-col>
                                <v-col cols="2">
                                    <v-tooltip text="File is too big">
                                        <template #activator="{ props: activatorProps }">
                                            <v-icon
                                                v-bind="activatorProps"
                                                icon="mdi-cancel"
                                                color="error2"
                                            />
                                        </template>
                                    </v-tooltip>
                                </v-col>
                            </v-row>
                        </v-expansion-panel-text>
                        <v-divider />
                    </v-expansion-panel>
                </v-expansion-panels>
            </v-col>
        </v-row>
    </v-snackbar>
</template>

<script setup lang="ts">
import {
    VSnackbar,
    VRow,
    VCol,
    VExpansionPanels,
    VExpansionPanel,
    VExpansionPanelTitle,
    VProgressLinear,
    VExpansionPanelText,
    VTooltip,
    VIcon,
    VDivider,
    VProgressCircular,
} from 'vuetify/components';

const props = defineProps<{
    onCancel: () => void,
}>();
</script>