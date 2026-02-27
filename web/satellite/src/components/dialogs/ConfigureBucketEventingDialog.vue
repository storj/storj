// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        max-width="550px"
        transition="fade-transition"
        :persistent="isLoading"
    >
        <v-card :loading="isLoading">
            <v-card-item class="pa-6">
                <template #prepend>
                    <v-sheet
                        class="border-sm d-flex justify-center align-center"
                        width="40"
                        height="40"
                        rounded="lg"
                    >
                        <component :is="Bell" :size="18" />
                    </v-sheet>
                </template>
                <v-card-title class="font-weight-bold">
                    Configure Bucket Eventing
                </v-card-title>
                <template #append>
                    <v-btn
                        icon="$close"
                        variant="text"
                        size="small"
                        color="default"
                        :disabled="isLoading"
                        @click="model = false"
                    />
                </template>
            </v-card-item>

            <v-divider />

            <v-card-item class="pa-0">
                <v-alert v-if="!hasSatelliteManagedEncryption" type="error" variant="tonal" class="ma-6">
                    Bucket eventing requires satellite-managed encryption to be enabled. Create a new project with satellite-managed encryption to use this feature.
                </v-alert>
                <v-alert v-else type="info" variant="tonal" class="ma-6 mb-0">
                    Configure event notifications to publish messages to Google Cloud Pub/Sub when specific events occur in this bucket.
                </v-alert>
                <!-- Configuration form -->
                <v-form v-if="hasSatelliteManagedEncryption" v-model="formValid" class="pa-6 pb-3" @submit.prevent>
                    <v-text-field
                        v-model="config.topicArn"
                        label="GCP Pub/Sub Topic"
                        placeholder="projects/PROJECT_ID/topics/TOPIC_ID"
                        :rules="topicArnRules"
                        hint="Format: projects/PROJECT_ID/topics/TOPIC_ID"
                        persistent-hint
                        variant="outlined"
                        required
                        class="mb-4"
                    />

                    <p class="text-subtitle-2 font-weight-bold mb-2">
                        Event Types *
                    </p>
                    <v-alert v-if="!config.events.length" type="warning" variant="tonal" density="compact" class="mb-4">
                        Select at least one event type
                    </v-alert>

                    <v-expansion-panels :elevation="0" variant="accordion" multiple flat>
                        <v-expansion-panel rounded="lg" hide-actions>
                            <v-expansion-panel-title v-slot="{ expanded }">
                                <div class="d-flex align-center">
                                    <v-checkbox
                                        :model-value="allCreatedChecked"
                                        :indeterminate="createdIndeterminate"
                                        density="compact"
                                        hide-details
                                        @click.stop
                                        @update:model-value="toggleAllCreated"
                                    />
                                    <span class="ml-2 mr-4">Object Created</span>
                                    <v-icon :icon="expanded ? ChevronUp : ChevronDown" />
                                </div>
                            </v-expansion-panel-title>
                            <v-expansion-panel-text>
                                <div class="ml-6">
                                    <v-checkbox
                                        :model-value="config.events.includes(EventType.ObjectCreatedPut)"
                                        label="Object Created (Put)"
                                        density="compact"
                                        hide-details
                                        @update:model-value="toggleEvent(EventType.ObjectCreatedPut, $event)"
                                    />
                                    <v-checkbox
                                        :model-value="config.events.includes(EventType.ObjectCreatedCopy)"
                                        label="Object Created (Copy)"
                                        density="compact"
                                        hide-details
                                        @update:model-value="toggleEvent(EventType.ObjectCreatedCopy, $event)"
                                    />
                                </div>
                            </v-expansion-panel-text>
                        </v-expansion-panel>

                        <v-expansion-panel rounded="lg" hide-actions>
                            <v-expansion-panel-title v-slot="{ expanded }">
                                <div class="d-flex align-center">
                                    <v-checkbox
                                        :model-value="allRemovedChecked"
                                        :indeterminate="removedIndeterminate"
                                        density="compact"
                                        hide-details
                                        @click.stop
                                        @update:model-value="toggleAllRemoved"
                                    />
                                    <span class="mx-2">Object Removed</span>
                                    <v-icon :icon="expanded ? ChevronUp : ChevronDown" />
                                </div>
                            </v-expansion-panel-title>
                            <v-expansion-panel-text>
                                <div class="ml-6">
                                    <v-checkbox
                                        :model-value="config.events.includes(EventType.ObjectRemovedDelete)"
                                        label="Object Removed (Delete)"
                                        density="compact"
                                        hide-details
                                        @update:model-value="toggleEvent(EventType.ObjectRemovedDelete, $event)"
                                    />
                                    <v-checkbox
                                        :model-value="config.events.includes(EventType.ObjectRemovedDeleteMarkerCreated)"
                                        label="Object Removed (Delete Marker)"
                                        density="compact"
                                        hide-details
                                        @update:model-value="toggleEvent(EventType.ObjectRemovedDeleteMarkerCreated, $event)"
                                    />
                                </div>
                            </v-expansion-panel-text>
                        </v-expansion-panel>
                    </v-expansion-panels>

                    <p class="text-subtitle-2 font-weight-bold mb-2 mt-4">
                        Filter Rules (Optional)
                        <v-tooltip text="A prefix of 'images/' and suffix of '.png' will match the object key 'images/logo.png'">
                            <template #activator="{ props }">
                                <v-icon class="ml-1" :icon="Info" size="16" v-bind="props" />
                            </template>
                        </v-tooltip>
                    </p>
                    <p class="text-body-2 text-medium-emphasis mb-6">
                        Match objects that start with [prefix] and end with [suffix].
                    </p>

                    <v-row class="mb-4">
                        <v-col>
                            <v-text-field
                                v-model="config.filterPrefix"
                                label="Object Key Prefix"
                                placeholder="Any prefix"
                                variant="outlined"
                                hide-details="auto"
                            />
                        </v-col>
                        <v-col>
                            <v-text-field
                                v-model="config.filterSuffix"
                                label="Object Key Suffix"
                                placeholder="Any suffix"
                                variant="outlined"
                                hide-details="auto"
                            />
                        </v-col>
                    </v-row>
                </v-form>
            </v-card-item>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col v-if="!hasSatelliteManagedEncryption">
                        <v-btn
                            color="primary"
                            variant="flat"
                            block
                            @click="() => {
                                model = false;
                                appStore.toggleCreateProjectDialog(true);
                            }"
                        >
                            Create Project
                        </v-btn>
                    </v-col>

                    <template v-if="hasSatelliteManagedEncryption">
                        <v-col>
                            <v-btn
                                variant="outlined"
                                color="default"
                                block
                                :disabled="isLoading"
                                @click="model = false"
                            >
                                Cancel
                            </v-btn>
                        </v-col>
                        <v-col>
                            <v-btn
                                color="primary"
                                variant="flat"
                                block
                                :loading="isLoading"
                                :disabled="!canSave"
                                @click="save()"
                            >
                                Save
                            </v-btn>
                        </v-col>
                    </template>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import {
    VAlert,
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
    VCardTitle,
    VCheckbox,
    VCol,
    VDialog,
    VDivider,
    VExpansionPanel,
    VExpansionPanelText,
    VExpansionPanelTitle,
    VExpansionPanels,
    VForm,
    VIcon,
    VRow,
    VSheet,
    VTextField,
    VTooltip,
} from 'vuetify/components';
import { Bell, ChevronDown, ChevronUp, Info } from 'lucide-vue-next';

import { BucketNotificationConfig, EventType } from '@/types/eventing';
import { useEventing } from '@/composables/useEventing';
import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useLoading } from '@/composables/useLoading';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useAppStore } from '@/store/modules/appStore';
import { ARN_PREFIX, convertArnToTopicName, parseTopicName } from '@/utils/eventing';

const appStore = useAppStore();
const projectStore = useProjectsStore();

const { getNotificationConfig, updateNotificationConfig } = useEventing();
const { withLoading, isLoading } = useLoading();
const notify = useNotify();

const props = defineProps<{
    bucketName: string;
}>();

const model = defineModel<boolean>({ required: true });

const emit = defineEmits<{
    updated: [];
}>();

const config = ref<BucketNotificationConfig>({ topicArn: '', events: [], filterPrefix: '', filterSuffix: '' });
const formValid = ref(false);

const topicArnRules = [
    (v: string) => !!v || 'Topic name is required',
    (v: string) => /^projects\/[^/]+\/topics\/[^/]+$/.test(v) || 'Invalid GCP Pub/Sub topic format. Expected: projects/PROJECT_ID/topics/TOPIC_ID',
];

const createdChildEvents = [EventType.ObjectCreatedPut, EventType.ObjectCreatedCopy];
const removedChildEvents = [EventType.ObjectRemovedDelete, EventType.ObjectRemovedDeleteMarkerCreated];

const allCreatedChecked = computed(() => createdChildEvents.every(e => config.value.events.includes(e)));
const allRemovedChecked = computed(() => removedChildEvents.every(e => config.value.events.includes(e)));

const createdIndeterminate = computed(() => {
    const count = createdChildEvents.filter(e => config.value.events.includes(e)).length;
    return count > 0 && count < createdChildEvents.length;
});
const removedIndeterminate = computed(() => {
    const count = removedChildEvents.filter(e => config.value.events.includes(e)).length;
    return count > 0 && count < removedChildEvents.length;
});

const canSave = computed(() => formValid.value && config.value.events.length > 0);

const hasSatelliteManagedEncryption = computed<boolean>(() => projectStore.selectedProjectConfig.hasManagedPassphrase);

function toggleEvent(eventType: EventType, checked: boolean | null) {
    if (checked && !config.value.events.includes(eventType)) {
        config.value.events.push(eventType);
    } else if (!checked) {
        const index = config.value.events.indexOf(eventType);
        if (index !== -1) config.value.events.splice(index, 1);
    }
}

function toggleAllCreated(checked: boolean | null) {
    config.value.events = config.value.events.filter(e => !createdChildEvents.includes(e));
    if (checked) {
        config.value.events.push(...createdChildEvents);
    }
}

function toggleAllRemoved(checked: boolean | null) {
    config.value.events = config.value.events.filter(e => !removedChildEvents.includes(e));
    if (checked) {
        config.value.events.push(...removedChildEvents);
    }
}

function loadConfiguration() {
    if (!props.bucketName || !hasSatelliteManagedEncryption.value) return;

    withLoading(async () => {
        try {
            const result = await getNotificationConfig(props.bucketName);
            const firstConfig = result.topicConfigurations?.[0] || null;
            if (!firstConfig) return;

            // Convert topic ARN to fully-qualified name
            firstConfig.topicArn = convertArnToTopicName(firstConfig.topicArn);

            // Expand wildcards into individual child events for editing.
            const events: EventType[] = firstConfig.events.filter(e => e !== EventType.ObjectCreatedAll && e !== EventType.ObjectRemovedAll);
            if (firstConfig.events.includes(EventType.ObjectCreatedAll)) {
                events.push(...createdChildEvents.filter(e => !events.includes(e)));
            }
            if (firstConfig.events.includes(EventType.ObjectRemovedAll)) {
                events.push(...removedChildEvents.filter(e => !events.includes(e)));
            }
            firstConfig.events = events;

            config.value = { ...firstConfig };
        } catch (error) {
            if (error instanceof Error && error.message.includes('Deserialization error')) {
                // No existing configuration
                return;
            }

            notify.notifyError(error, AnalyticsErrorEventSource.BUCKET_EVENTING_CONFIG_DIALOG);
        }
    });
}

function save() {
    withLoading(async () => {
        try {
            const { projectId, topicId } = parseTopicName(config.value.topicArn);
            const cfg = { ...config.value };
            cfg.topicArn = `${ARN_PREFIX}${projectId}:${topicId}`;

            // Collapse child events into wildcards when all children are selected.
            if (allCreatedChecked.value) {
                cfg.events = cfg.events.filter(e => !createdChildEvents.includes(e));
                cfg.events.push(EventType.ObjectCreatedAll);
            }
            if (allRemovedChecked.value) {
                cfg.events = cfg.events.filter(e => !removedChildEvents.includes(e));
                cfg.events.push(EventType.ObjectRemovedAll);
            }

            await updateNotificationConfig(props.bucketName, cfg);
            notify.success(`Bucket eventing configuration updated successfully`);
            model.value = false;
            emit('updated');
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.BUCKET_EVENTING_CONFIG_DIALOG);
        }
    });
}

watch(model, (isOpen) => {
    if (isOpen) loadConfiguration();
    else config.value = { topicArn: '', events: [], filterPrefix: '', filterSuffix: '' };
});
</script>
