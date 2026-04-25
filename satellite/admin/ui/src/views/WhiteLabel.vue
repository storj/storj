// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container fluid>
        <v-row justify="center">
            <v-col cols="12" md="10" lg="8" xl="6">
                <v-card
                    :loading="isLoading"
                    title="Tenant Whitelabel Config"
                    subtitle="View and edit the per-tenant whitelabel configuration. Stored values override the YAML SingleWhiteLabel config on console startup."
                    variant="flat" rounded="xlg" border
                >
                    <v-card-text>
                        <v-row dense>
                            <v-col cols="12" md="8">
                                <v-text-field
                                    v-model="tenantID"
                                    :readonly="isTenantScoped"
                                    :hint="isTenantScoped ? 'This admin is scoped to a single tenant.' : 'Enter the tenant ID to view or edit.'"
                                    persistent-hint
                                    label="Tenant ID"
                                    density="compact"
                                    rounded="lg"
                                    variant="outlined"
                                />
                            </v-col>
                            <v-col cols="12" md="4" class="d-flex align-start">
                                <v-btn
                                    :disabled="!tenantID || isLoading"
                                    color="primary"
                                    rounded="lg"
                                    variant="outlined"
                                    @click="load"
                                >
                                    Load
                                </v-btn>
                            </v-col>
                        </v-row>

                        <v-row v-if="loadedTenantID" dense>
                            <v-col cols="12" class="text-caption text-medium-emphasis">
                                <span v-if="lastUpdatedAt">Last updated: {{ lastUpdatedAt }}</span>
                                <span v-else>No row exists yet for this tenant. Saving will create one.</span>
                            </v-col>
                        </v-row>

                        <v-textarea
                            v-model="configYaml"
                            label="Config (YAML)"
                            rows="20"
                            variant="outlined"
                            rounded="lg"
                            class="mt-2"
                            :disabled="!loadedTenantID || !canUpdate"
                        />
                    </v-card-text>

                    <v-card-actions class="px-4 pb-4">
                        <v-spacer />
                        <v-btn
                            v-if="loadedTenantID && canUpdate"
                            :loading="isSaving"
                            color="primary"
                            rounded="lg"
                            variant="flat"
                            @click="save"
                        >
                            Save
                        </v-btn>
                    </v-card-actions>
                </v-card>
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import {
    VBtn,
    VCard,
    VCardActions,
    VCardText,
    VCol,
    VContainer,
    VRow,
    VSpacer,
    VTextField,
    VTextarea,
} from 'vuetify/components';

import {
    APIError,
    UpdateTenantWhiteLabelConfigRequest,
    WhiteLabelManagementHttpApiV1,
} from '@/api/client.gen';
import { useAppStore } from '@/store/app';
import { useNotificationsStore } from '@/store/notifications';
import { useLoading } from '@/composables/useLoading';

const appStore = useAppStore();
const notify = useNotificationsStore();
const { isLoading } = useLoading();

const api = new WhiteLabelManagementHttpApiV1();

const tenantID = ref<string>('');
const loadedTenantID = ref<string>('');
const configYaml = ref<string>('');
const lastUpdatedAt = ref<string>('');
const isSaving = ref<boolean>(false);

const tenantScope = computed<string>(() => appStore.state.settings?.console?.tenantScope ?? '');
const isTenantScoped = computed<boolean>(() => tenantScope.value !== '');
const canUpdate = computed<boolean>(() => appStore.state.settings?.admin?.features?.whiteLabel?.update ?? false);

async function load(): Promise<void> {
    if (!tenantID.value) return;
    isLoading.value = true;
    try {
        const result = await api.getTenantWhiteLabelConfig(tenantID.value);
        loadedTenantID.value = result.tenantID;
        configYaml.value = result.configYAML ?? '';
        lastUpdatedAt.value = result.updatedAt ?? '';
    } catch (err) {
        if (err instanceof APIError && err.responseStatusCode === 404) {
            loadedTenantID.value = tenantID.value;
            configYaml.value = '';
            lastUpdatedAt.value = '';
            notify.notifySuccess('No config exists yet for this tenant. You can create one.');
        } else {
            notify.notifyError(`Failed to load config: ${err instanceof Error ? err.message : String(err)}`);
        }
    } finally {
        isLoading.value = false;
    }
}

async function save(): Promise<void> {
    isSaving.value = true;
    try {
        const req = new UpdateTenantWhiteLabelConfigRequest();
        req.configYAML = configYaml.value;
        const result = await api.updateTenantWhiteLabelConfig(req, loadedTenantID.value);
        configYaml.value = result.configYAML ?? '';
        lastUpdatedAt.value = result.updatedAt ?? '';
        notify.notifySuccess('Whitelabel config saved.');
    } catch (err) {
        notify.notifyError(`Failed to save config: ${err instanceof Error ? err.message : String(err)}`);
    } finally {
        isSaving.value = false;
    }
}

onMounted(() => {
    if (isTenantScoped.value) {
        tenantID.value = tenantScope.value;
        void load();
    }
});
</script>
