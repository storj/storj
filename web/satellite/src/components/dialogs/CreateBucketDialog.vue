// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="auto"
        max-width="450px"
        transition="fade-transition"
        persistent
        scrollable
    >
        <v-card ref="innerContent">
            <v-sheet>
                <v-card-item class="pa-6">
                    <template #prepend>
                        <v-sheet
                            class="border-sm d-flex justify-center align-center"
                            width="40"
                            height="40"
                            rounded="lg"
                        >
                            <svg width="18" height="18" viewBox="0 0 18 18" fill="none" xmlns="http://www.w3.org/2000/svg">
                                <path
                                    d="M16.3549 7.54666C17.762 8.95376 17.7749 11.227 16.3778 12.624C15.8118 13.1901 15.1018 13.5247 14.3639 13.6289L14.1106 15.0638C14.092 16.1897 11.7878 17.1 8.94814 17.1C6.12547 17.1 3.83191 16.2006 3.78632 15.0841L3.78589 15.0638L1.84471 4.06597C1.82286 3.98819 1.80888 3.90946 1.8031 3.82992L1.80005 3.81221L1.80195 3.81231C1.80068 3.79028 1.80005 3.76818 1.80005 3.74602C1.80005 2.17422 5.00036 0.900024 8.94814 0.900024C12.8959 0.900024 16.0962 2.17422 16.0962 3.74602C16.0962 3.76818 16.0956 3.79028 16.0943 3.81231L16.0962 3.81221L16.0931 3.83111C16.0873 3.90975 16.0735 3.98759 16.052 4.06451L15.5749 6.76662L16.3549 7.54666ZM14.2962 5.63437C12.9868 6.22183 11.076 6.59202 8.94814 6.59202C6.82032 6.59202 4.90965 6.22185 3.6002 5.63443L5.00729 13.6077L5.23735 14.8286L5.25867 14.8452C5.37899 14.9354 5.56521 15.0371 5.80702 15.1351L5.85612 15.1546C6.63558 15.4594 7.74625 15.6439 8.94814 15.6439C10.157 15.6439 11.2733 15.4573 12.0528 15.1497C12.3338 15.0388 12.5432 14.9223 12.6661 14.8231L12.6761 14.8148L12.902 13.5348C12.3339 13.3787 11.7956 13.0812 11.3429 12.6429L11.3005 12.6011L8.37494 9.67559C8.09062 9.39127 8.09062 8.93029 8.37494 8.64597C8.65232 8.36859 9.09785 8.36182 9.38344 8.62568L9.40455 8.64597L12.3301 11.5715C12.5718 11.8132 12.8556 11.9861 13.157 12.0901L14.2962 5.63437ZM15.2661 8.51698L14.6409 12.0597C14.899 11.9575 15.1403 11.8024 15.3482 11.5944C16.1642 10.7784 16.1664 9.44942 15.355 8.60652L15.3253 8.57627L15.2661 8.51698ZM8.94814 2.35612C7.20131 2.35612 5.58131 2.62893 4.43229 3.08641C3.93857 3.28298 3.57123 3.49947 3.35982 3.69848C3.34635 3.71116 3.33405 3.72325 3.32289 3.73469L3.31227 3.74589L3.33148 3.76606L3.35982 3.79357C3.57123 3.99258 3.93857 4.20906 4.43229 4.40564C5.58131 4.86312 7.20131 5.13593 8.94814 5.13593C10.695 5.13593 12.315 4.86312 13.464 4.40564C13.9577 4.20906 14.325 3.99258 14.5365 3.79357C14.5499 3.78089 14.5622 3.7688 14.5734 3.75735L14.5841 3.74589L14.5648 3.72599L14.5365 3.69848C14.325 3.49947 13.9577 3.28298 13.464 3.08641C12.315 2.62893 10.695 2.35612 8.94814 2.35612Z"
                                    fill="currentColor"
                                />
                            </svg>
                        </v-sheet>
                    </template>

                    <v-card-title class="font-weight-bold">
                        New Bucket
                    </v-card-title>

                    <v-card-subtitle class="text-caption pb-0">
                        Step {{ stepNumber }}: {{ stepName }}
                    </v-card-subtitle>

                    <template #append>
                        <v-btn
                            :icon="X"
                            variant="text"
                            size="small"
                            color="default"
                            :disabled="isLoading"
                            @click="model = false"
                        />
                    </template>
                </v-card-item>
            </v-sheet>

            <v-divider />

            <v-card-text class="pa-0">
                <v-window v-model="step" :touch="false">
                    <v-window-item :value="CreateStep.Name">
                        <v-form v-model="formValid" class="pa-6 pb-3" @submit.prevent>
                            <v-row>
                                <v-col>
                                    <p>
                                        Buckets are used to store and organize your objects. Enter a bucket name using lowercase
                                        characters.
                                    </p>
                                    <v-text-field
                                        id="Bucket Name"
                                        v-model="bucketName"
                                        variant="outlined"
                                        :rules="bucketNameRules"
                                        label="Bucket name"
                                        placeholder="my-bucket"
                                        hint="Allowed characters [a-z] [0-9] [-.]"
                                        :hide-details="false"
                                        required
                                        autofocus
                                        class="mt-7 mb-3"
                                        minlength="3"
                                        maxlength="63"
                                    />
                                    <v-alert v-if="dotsInName" variant="tonal" type="warning" rounded="lg">
                                        Using dots (.) in the bucket name can cause incompatibility.
                                    </v-alert>
                                </v-col>
                            </v-row>
                        </v-form>
                    </v-window-item>

                    <v-window-item v-if="selfPlacementEnabled" :value="CreateStep.Location">
                        <v-form v-model="formValid" class="pa-6" @submit.prevent>
                            <v-row>
                                <v-col cols="12">
                                    <p class="font-weight-bold mb-2">
                                        Choose {{ showNewPricingTiers ? 'Storage Tier' : 'Data Location' }}
                                    </p>
                                    <p v-if="showNewPricingTiers">
                                        Choose the storage type that matches your needs.
                                    </p>
                                    <p v-else>
                                        Allows you to select the whole world, or specific regions to store the data you upload in this
                                        bucket.
                                    </p>
                                    <v-chip-group
                                        v-model="bucketLocation"
                                        filter
                                        selected-class="font-weight-bold"
                                        class="mt-2 mb-2"
                                        mandatory
                                        column
                                    >
                                        <v-chip
                                            v-for="placement in placementDetails"
                                            :key="placement.id"
                                            variant="outlined"
                                            filter
                                            :value="placement.idName"
                                            color="primary"
                                        >
                                            {{ placement.name }}
                                        </v-chip>
                                    </v-chip-group>

                                    <v-alert v-if="bucketLocation" variant="tonal" color="secondary" width="auto">
                                        <template v-for="placement in placementDetails">
                                            <template v-if="bucketLocation === placement.idName">
                                                <div :key="placement.id">
                                                    <p class="text-subtitle-2 font-weight-bold">{{ placement.title }}</p>
                                                    <p class="text-subtitle-2 mb-2">{{ placement.description }}</p>
                                                </div>
                                            </template>
                                        </template>
                                        <a v-if="showNewPricingTiers" href="https://storj.dev/dcs/pricing" target="_blank" rel="noopener noreferrer">
                                            View Pricing
                                        </a>
                                        <template v-else>
                                            <v-chip v-if="pricingForLocation" variant="tonal" class="mr-1">
                                                {{ formatToGBDollars(pricingForLocation.storageMBMonthCents) }}/GB-month stored
                                                <template v-if="isGettingPricing" #prepend>
                                                    <v-progress-circular indeterminate :size="20" />
                                                </template>
                                            </v-chip>
                                            <v-chip v-if="pricingForLocation" variant="tonal">
                                                {{ formatToGBDollars(pricingForLocation.egressMBCents) }}/GB download
                                                <template v-if="isGettingPricing" #prepend>
                                                    <v-progress-circular indeterminate :size="20" />
                                                </template>
                                            </v-chip>
                                        </template>

                                        <template v-if="selectedPlacement?.pending">
                                            <v-alert color="info" variant="tonal" density="comfortable" class="my-3">
                                                <p class="text-body-2 font-weight-medium">
                                                    {{ bucketLocationName }} is coming soon! Share your storage needs and we'll notify you when it becomes available for your account.
                                                </p>
                                            </v-alert>

                                            <v-select
                                                v-model="selectStorageNeeds"
                                                :items="selectStorageOptions"
                                                label="Expected Data Stored"
                                                variant="outlined"
                                                density="comfortable"
                                                hide-details
                                                class="mt-5"
                                            />

                                            <v-expand-transition>
                                                <v-alert
                                                    v-if="isWaitlistJoined(selectedPlacement.id)"
                                                    color="success"
                                                    variant="tonal"
                                                    class="mt-3"
                                                >
                                                    Thanks for your interest! We'll notify you when {{ configStore.brandName }} Select becomes available for your account.
                                                </v-alert>
                                            </v-expand-transition>

                                            <v-btn
                                                v-if="!isWaitlistJoined(selectedPlacement.id)"
                                                block
                                                color="primary"
                                                variant="flat"
                                                class="mt-4"
                                                :loading="selectSubmitting"
                                                :disabled="!selectStorageNeeds"
                                                @click="joinPlacementWaitlist"
                                            >
                                                Notify Me When Available
                                            </v-btn>
                                        </template>
                                    </v-alert>
                                </v-col>
                            </v-row>
                        </v-form>
                    </v-window-item>

                    <v-window-item :value="CreateStep.ObjectLock">
                        <v-form v-model="formValid" class="pa-6" @submit.prevent>
                            <v-row>
                                <v-col>
                                    <p class="font-weight-bold mb-2">Do you need object lock?</p>
                                    <p>
                                        Enabling object lock will prevent objects from being deleted or overwritten for a specified period
                                        of time.
                                    </p>
                                    <v-chip-group
                                        v-model="enableObjectLock"
                                        filter
                                        selected-class="font-weight-bold"
                                        class="mt-2 mb-2"
                                        mandatory
                                    >
                                        <v-chip
                                            variant="outlined"
                                            filter
                                            color="info"
                                            :value="false"
                                        >
                                            No
                                        </v-chip>
                                        <v-chip
                                            variant="outlined"
                                            filter
                                            color="info"
                                            :value="true"
                                        >
                                            Yes
                                        </v-chip>
                                    </v-chip-group>
                                    <SetDefaultObjectLockConfig
                                        v-if="enableObjectLock"
                                        v-model:default-retention-period="defaultRetentionPeriod"
                                        v-model:default-retention-mode="defaultRetentionMode"
                                        v-model:period-unit="defaultRetentionPeriodUnit"
                                    />
                                    <v-alert v-else variant="tonal" color="default">
                                        <p class="font-weight-bold text-body-2 mb-1">Object Lock Disabled (Default)</p>
                                        <p class="text-subtitle-2">Objects can be deleted or overwritten.</p>
                                    </v-alert>
                                </v-col>
                            </v-row>
                        </v-form>
                    </v-window-item>

                    <v-window-item :value="CreateStep.Versioning">
                        <v-form v-model="formValid" class="pa-6" @submit.prevent>
                            <v-row>
                                <v-col>
                                    <p class="font-weight-bold mb-2">Do you want to enable versioning?</p>
                                    <p>
                                        Enabling object versioning allows you to preserve, retrieve, and restore previous versions of an
                                        object, offering protection against unintentional modifications or deletions.
                                    </p>
                                    <v-chip-group
                                        v-model="enableVersioning"
                                        :disabled="enableObjectLock"
                                        filter
                                        selected-class="font-weight-bold"
                                        class="mt-2 mb-2"
                                        mandatory
                                    >
                                        <v-chip
                                            v-if="!enableObjectLock"
                                            variant="outlined"
                                            filter
                                            color="info"
                                            :value="false"
                                        >
                                            Disabled
                                        </v-chip>
                                        <v-chip
                                            variant="outlined"
                                            filter
                                            color="info"
                                            :value="true"
                                        >
                                            Enabled
                                        </v-chip>
                                    </v-chip-group>
                                    <v-alert v-if="enableObjectLock" variant="tonal" color="default" class="mb-3">
                                        <p class="text-subtitle-2 font-weight-bold">Versioning must be enabled for object lock to work.</p>
                                    </v-alert>
                                    <v-alert v-if="enableVersioning" variant="tonal" color="default">
                                        <p class="text-subtitle-2">
                                            Keep multiple versions of each object in the same bucket. Additional
                                            storage costs apply for each version.
                                        </p>
                                    </v-alert>
                                    <v-alert v-else variant="tonal" color="default">
                                        <p class="text-subtitle-2">
                                            Uploading an object with the same name will overwrite the existing object
                                            in this bucket.
                                        </p>
                                    </v-alert>
                                </v-col>
                            </v-row>
                        </v-form>
                    </v-window-item>

                    <v-window-item :value="CreateStep.Confirmation">
                        <v-card-text class="pa-6">
                            <v-row>
                                <v-col>
                                    <p class="mb-4">You are about to create a new bucket with the following settings:</p>
                                    <p>Name:</p>
                                    <v-chip
                                        variant="tonal"
                                        value="Disabled"
                                        color="default"
                                        class="mt-1 mb-4 font-weight-bold"
                                    >
                                        {{ bucketName }}
                                    </v-chip>

                                    <template v-if="selfPlacementEnabled">
                                        <p v-if="showNewPricingTiers">Storage Tier:</p>
                                        <p v-else>Location:</p>
                                        <v-chip
                                            variant="tonal"
                                            value="Disabled"
                                            color="default"
                                            class="mt-1 mb-4 font-weight-bold"
                                        >
                                            {{ bucketLocationName }}
                                        </v-chip>
                                    </template>

                                    <template v-if="objectLockUIEnabled">
                                        <p>Object Lock:</p>
                                        <v-chip
                                            variant="tonal"
                                            value="Disabled"
                                            color="default"
                                            class="mt-1 mb-4 font-weight-bold"
                                        >
                                            {{ enableObjectLock ? 'Enabled' : 'Disabled' }}
                                        </v-chip>
                                        <p>Default Retention Mode:</p>
                                        <v-chip
                                            variant="tonal"
                                            color="default"
                                            class="mt-1 mb-4 font-weight-bold text-capitalize"
                                        >
                                            {{ defaultRetentionMode?.toLowerCase() ?? NO_MODE_SET }}
                                        </v-chip>
                                        <p>Default Retention Period:</p>
                                        <v-chip
                                            variant="tonal"
                                            color="default"
                                            class="mt-1 mb-4 font-weight-bold"
                                        >
                                            {{ defaultRetPeriodResult }}
                                        </v-chip>
                                    </template>

                                    <template v-if="versioningUIEnabled">
                                        <p>Versioning:</p>
                                        <v-chip
                                            variant="tonal"
                                            value="Disabled"
                                            color="default"
                                            class="mt-1 font-weight-bold"
                                        >
                                            {{ enableVersioning ? 'Enabled' : 'Disabled' }}
                                        </v-chip>
                                    </template>
                                </v-col>
                            </v-row>
                        </v-card-text>
                    </v-window-item>

                    <v-window-item :value="CreateStep.Success">
                        <v-card-text class="pa-6">
                            <v-row>
                                <v-col>
                                    <p>
                                        <strong>
                                            <v-icon :icon="Check" size="small" />
                                            Bucket successfully created.</strong>
                                    </p>
                                    <v-chip
                                        variant="tonal"
                                        value="Disabled"
                                        color="primary"
                                        class="my-4 font-weight-bold"
                                    >
                                        {{ bucketName }}
                                    </v-chip>
                                    <p>
                                        You can open the bucket to start uploading objects, or close this dialog to return to all buckets.
                                    </p>
                                </v-col>
                            </v-row>
                        </v-card-text>
                    </v-window-item>
                </v-window>
            </v-card-text>

            <v-divider />

            <v-card-actions v-if="!selectedPlacement?.pending" class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn :disabled="isLoading" variant="outlined" color="default" block @click="toPrevStep">
                            {{ stepInfos[step].prevText }}
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            :disabled="formInvalid"
                            :loading="isLoading"
                            :append-icon="ArrowRight"
                            color="primary"
                            variant="flat"
                            block
                            @click="toNextStep"
                        >
                            {{ stepInfos[step].nextText }}
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { Component, computed, ref, watch, watchEffect } from 'vue';
import {
    VAlert,
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
    VCardSubtitle,
    VCardText,
    VCardTitle,
    VChip,
    VChipGroup,
    VCol,
    VDialog,
    VDivider,
    VExpandTransition,
    VForm,
    VIcon,
    VProgressCircular,
    VRow,
    VSelect,
    VSheet,
    VTextField,
    VWindow,
    VWindowItem,
} from 'vuetify/components';
import { ArrowRight, Check, X } from 'lucide-vue-next';
import { useRouter } from 'vue-router';

import { CENTS_MB_TO_DOLLARS_GB_SHIFT, decimalShift, formatPrice } from '@/utils/strings';
import { useLoading } from '@/composables/useLoading';
import { useConfigStore } from '@/store/modules/configStore';
import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { ClientType, useBucketsStore } from '@/store/modules/bucketsStore';
import { LocalData } from '@/utils/localData';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';
import { StepInfo, ValidationRule } from '@/types/common';
import { Versioning } from '@/types/versioning';
import { ROUTES } from '@/router';
import { DefaultObjectLockPeriodUnit, NO_MODE_SET, ObjLockMode } from '@/types/objectLock';
import { useBillingStore } from '@/store/modules/billingStore';
import { UsagePriceModel } from '@/types/payments';
import { PlacementDetails } from '@/types/buckets';
import { useAccessGrantWorker } from '@/composables/useAccessGrantWorker';
import { useUsersStore } from '@/store/modules/usersStore';

import SetDefaultObjectLockConfig from '@/components/dialogs/defaultBucketLockConfig/SetDefaultObjectLockConfig.vue';

enum CreateStep {
    Name = 1,
    Location,
    ObjectLock,
    Versioning,
    Confirmation,
    Success,
}

const { setPermissions, generateAccess } = useAccessGrantWorker();
const { isLoading, withLoading } = useLoading();
const notify = useNotify();
const router = useRouter();

const agStore = useAccessGrantsStore();
const userStore = useUsersStore();
const projectsStore = useProjectsStore();
const billingStore = useBillingStore();
const bucketsStore = useBucketsStore();
const analyticsStore = useAnalyticsStore();
const configStore = useConfigStore();

const stepInfos = {
    [CreateStep.Name]: new StepInfo<CreateStep>({
        prev: undefined,
        next: () => {
            if (selfPlacementEnabled.value) return CreateStep.Location;
            if (objectLockUIEnabled.value) return CreateStep.ObjectLock;
            if (allowVersioningStep.value) return CreateStep.Versioning;
            return CreateStep.Success;
        },
        beforeNext: async () => {
            if (selfPlacementEnabled.value || objectLockUIEnabled.value || allowVersioningStep.value) return;
            await onCreate();
        },
        validate: (): boolean => {
            return formValid.value;
        },
        nextText: () => objectLockUIEnabled.value || allowVersioningStep.value ? 'Next' : 'Create Bucket',
        noRef: true,
    }),
    [CreateStep.Location]: new StepInfo<CreateStep>({
        prev: () => CreateStep.Name,
        next: () => {
            if (objectLockUIEnabled.value) return CreateStep.ObjectLock;
            if (allowVersioningStep.value) return CreateStep.Versioning;
            return CreateStep.Success;
        },
        beforeNext: async () => {
            if (objectLockUIEnabled.value || allowVersioningStep.value) return;
            await onCreate();
        },
        validate: (): boolean => {
            return !!bucketLocation.value;
        },
        noRef: true,
    }),
    [CreateStep.ObjectLock]: new StepInfo<CreateStep>({
        prev: () => {
            if (selfPlacementEnabled.value) return CreateStep.Location;
            return CreateStep.Name;
        },
        next: () => {
            if (allowVersioningStep.value) return CreateStep.Versioning;
            return CreateStep.Confirmation;
        },
        beforePrev: () => {
            if (formValid.value) return;

            defaultRetentionMode.value = NO_MODE_SET;
            defaultRetentionPeriod.value = 0;
            defaultRetentionPeriodUnit.value = DefaultObjectLockPeriodUnit.DAYS;
        },
        noRef: true,
    }),
    [CreateStep.Versioning]: new StepInfo<CreateStep>({
        prev: () => {
            if (objectLockUIEnabled.value) return CreateStep.ObjectLock;
            if (selfPlacementEnabled.value) return CreateStep.Location;
            return CreateStep.Name;
        },

        next: () => CreateStep.Confirmation,
        noRef: true,
    }),
    [CreateStep.Confirmation]: new StepInfo<CreateStep>({
        prev: () => {
            if (allowVersioningStep.value) return CreateStep.Versioning;
            if (objectLockUIEnabled.value) return CreateStep.ObjectLock;
            if (selfPlacementEnabled.value) return CreateStep.Location;
            return CreateStep.Name;
        },
        beforeNext: onCreate,
        next: () => CreateStep.Success,
        nextText: 'Create Bucket',
        noRef: true,
    }),
    [CreateStep.Success]: new StepInfo<CreateStep>({
        prevText: 'Close',
        nextText: 'Open Bucket',
        noRef: true,
    }),
};

// Copied from here https://github.com/storj/storj/blob/f6646b0e88700b5e7113a76a8d07bf346b59185a/satellite/metainfo/validation.go#L38
const ipRegexp = /^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$/;

const model = defineModel<boolean>({ required: true });

const emit = defineEmits<{
    (event: 'created', value: string): void,
}>();

const step = ref<CreateStep>(CreateStep.Name);
const stepNumber = ref<number>(1);
const innerContent = ref<Component | null>(null);
const formValid = ref<boolean>(false);
const enableVersioning = ref<boolean>(false);
const enableObjectLock = ref<boolean>(false);
const bucketName = ref<string>('');
const defaultRetentionMode = ref<ObjLockMode | typeof NO_MODE_SET>(NO_MODE_SET);
const defaultRetentionPeriod = ref<number>(0);
const defaultRetentionPeriodUnit = ref<DefaultObjectLockPeriodUnit>(DefaultObjectLockPeriodUnit.DAYS);
const bucketLocation = ref<string>();
const isGettingPricing = ref<boolean>(false);
const pricingForLocation = ref<UsagePriceModel>();

const selectSubmitting = ref(false);
const selectStorageNeeds = ref<string>();
const selectStorageOptions = ['< 1TB', '1-10TB', '10-50TB', '50-100TB', '> 100TB'];

const project = computed(() => projectsStore.state.selectedProject);
const projectConfig = computed(() => projectsStore.state.selectedProjectConfig);
const placementDetails = computed(() => projectConfig.value.availablePlacements || []);

const dotsInName = computed<boolean>(() => bucketName.value.includes('.'));

const defaultRetPeriodResult = computed<string>(() => {
    if (defaultRetentionPeriod.value === 0) return NO_MODE_SET;

    let unit = defaultRetentionPeriodUnit.value.toString();
    if (defaultRetentionPeriod.value === 1) {
        unit = unit.slice(0, -1);
    }

    return `${defaultRetentionPeriod.value} ${unit}`;
});

const showNewPricingTiers = computed<boolean>(() => configStore.state.config.showNewPricingTiers);

const selfPlacementEnabled = computed<boolean>(() => {
    if (!configStore.state.config.selfServePlacementSelectEnabled) return false;

    return (configStore.state.config.entitlementsEnabled || !project.value.placement) &&
        !!projectConfig.value.availablePlacements.length;
});

const selectedPlacement = computed<PlacementDetails | null>(() => {
    if (!bucketLocation.value) return null;
    return placementDetails.value.find(p => p.idName === bucketLocation.value) || null;
});

const bucketLocationName = computed<string>(() => {
    const details = selectedPlacement.value;
    if (!details) return '';

    return details.shortName || details.name;
});

const formInvalid = computed(() => !formValid.value || (selfPlacementEnabled.value && step.value === CreateStep.Location && !bucketLocation.value));

/**
 * Whether versioning has been enabled for current project.
 */
const versioningUIEnabled = computed<boolean>(() => configStore.state.config.versioningUIEnabled);

/**
 * Whether the versioning step should be shown.
 * Projects with versioning enabled as default should not have this step.
 */
const allowVersioningStep = computed<boolean>(() => {
    return versioningUIEnabled.value && project.value.versioning !== Versioning.Enabled;
});

/**
 * Whether object lock is enabled for current project.
 */
const objectLockUIEnabled = computed<boolean>(() => configStore.state.config.objectLockUIEnabled);

const bucketNameRules = computed((): ValidationRule<string>[] => {
    return [
        (value: string) => (!!value || 'Bucket name is required.'),
        (value: string) => ((value.length >= 3 && value.length <= 63) || 'Name should be between 3 and 63 characters length.'),
        (value: string) => {
            const labels = value.split('.');
            for (let i = 0; i < labels.length; i++) {
                const l = labels[i];
                if (!l.length) return 'Bucket name cannot start or end with a dot.';
                if (!/^[a-z0-9]$/.test(l[0])) return 'Bucket name must start with a lowercase letter or number.';
                if (!/^[a-z0-9]$/.test(l[l.length - 1])) return 'Bucket name must end with a lowercase letter or number.';
                if (!/^[a-z0-9-.]+$/.test(l)) return 'Bucket name can contain only lowercase letters, numbers or hyphens.';
            }
            return true;
        },
        (value: string) => (!ipRegexp.test(value) || 'Bucket name cannot be formatted as an IP address.'),
        (value: string) => (!allBucketNames.value.includes(value) || 'A bucket exists with this name.'),
    ];
});

const stepName = computed<string>(() => {
    switch (step.value) {
    case CreateStep.Name:
        return 'Name';
    case CreateStep.Location:
        return showNewPricingTiers.value ? 'Storage Tier' : 'Data Location';
    case CreateStep.ObjectLock:
        return 'Object Lock';
    case CreateStep.Versioning:
        return 'Object Versioning';
    case CreateStep.Confirmation:
        return 'Confirmation';
    case CreateStep.Success:
        return 'Bucket Successfully Created';
    default:
        return '';
    }
});

/**
 * Returns all bucket names from store.
 */
const allBucketNames = computed((): string[] => {
    return bucketsStore.state.allBucketNames;
});

/**
 * Returns condition if user has to be prompt for passphrase from store.
 */
const promptForPassphrase = computed((): boolean => {
    return bucketsStore.state.promptForPassphrase;
});

/**
 * Returns object browser api key from store.
 */
const apiKey = computed((): string => {
    return bucketsStore.state.apiKey;
});

/**
 * Returns edge credentials from store.
 */
const edgeCredentials = computed((): EdgeCredentials => {
    return bucketsStore.state.edgeCredentials;
});

/**
 * Returns edge credentials for bucket creation from store.
 */
const edgeCredentialsForCreate = computed((): EdgeCredentials => {
    return bucketsStore.state.edgeCredentialsForCreate;
});

/**
 * Indicates if bucket was created.
 */
const bucketWasCreated = computed((): boolean => {
    const status = LocalData.getBucketWasCreatedStatus();
    if (status !== null) {
        return status;
    }

    return false;
});

/**
 * Conditionally close dialog or go to previous step.
 */
function toPrevStep(): void {
    if (step.value === CreateStep.Success) {
        model.value = false;
        return;
    }
    const info = stepInfos[step.value];
    info.beforePrev?.();

    if (info.prev?.value) {
        step.value = info.prev.value;
        stepNumber.value--;
    } else {
        model.value = false;
    }
}

/**
 * Conditionally create bucket or go to next step.
 */
function toNextStep(): void {
    if (!formValid.value) return;

    if (step.value === CreateStep.Success) {
        openBucket();
        return;
    }
    const info = stepInfos[step.value];
    if (info?.validate?.() === false) {
        return;
    }
    withLoading(async () => {
        try {
            await info.beforeNext?.();
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.CREATE_BUCKET_MODAL);
            return;
        }
        if (info.next?.value) {
            step.value = info.next.value;
            stepNumber.value++;
        }
    });
}

function isWaitlistJoined(placementID: number): boolean {
    const placementWaitlistsJoined = userStore.state.settings.noticeDismissal.placementWaitlistsJoined || [];
    return placementWaitlistsJoined.includes(placementID);
}

async function joinPlacementWaitlist(): Promise<void> {
    if (!selectStorageNeeds.value) {
        notify.error('Storage needs is required');
    }
    selectSubmitting.value = true;
    try {
        await analyticsStore.joinPlacementWaitlist(selectStorageNeeds.value || '', selectedPlacement.value?.id || 0);
        await analyticsStore.ensureEventTriggered(AnalyticsEvent.JOIN_PLACEMENT_WAITLIST_FORM_SUBMITTED, {
            storageNeeds: selectStorageNeeds.value || '',
            placement: `${selectedPlacement.value?.id || 0}`,
        });

        await userStore.getSettings();
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.PLACEMENT_WAITLIST_FORM);
    } finally {
        selectSubmitting.value = false;
    }
}

/**
 * Navigates to bucket page.
 */
async function openBucket(): Promise<void> {
    bucketsStore.setFileComponentBucketName(bucketName.value);
    await router.push({
        name: ROUTES.Bucket.name,
        params: {
            browserPath: bucketsStore.state.fileComponentBucketName,
            id: projectsStore.state.selectedProject.urlId,
        },
    });
}

async function setObjectLockConfig(clientType: ClientType): Promise<void> {
    await bucketsStore.setObjectLockConfig(bucketName.value, clientType, {
        DefaultRetention: {
            Mode: defaultRetentionMode.value === NO_MODE_SET ? undefined : defaultRetentionMode.value,
            Days: defaultRetentionPeriodUnit.value === DefaultObjectLockPeriodUnit.DAYS ? defaultRetentionPeriod.value : undefined,
            Years: defaultRetentionPeriodUnit.value === DefaultObjectLockPeriodUnit.YEARS ? defaultRetentionPeriod.value : undefined,
        },
    });
}

/**
 * Validates provided bucket's name and creates a bucket.
 */
async function onCreate(): Promise<void> {
    const projectID = project.value.id;

    if (!promptForPassphrase.value) {
        if (!edgeCredentials.value.accessKeyId) {
            await bucketsStore.setS3Client(projectID);
        }
        await bucketsStore.createBucket({
            name: bucketName.value,
            enableObjectLock: enableObjectLock.value,
            enableVersioning: enableVersioning.value,
            placementName: bucketLocation.value,
        });
        if (enableObjectLock.value && defaultRetentionMode.value !== NO_MODE_SET) await setObjectLockConfig(ClientType.REGULAR);
        await bucketsStore.getBuckets(1, projectID);
        analyticsStore.eventTriggered(AnalyticsEvent.BUCKET_CREATED, { project_id: projectID });

        if (!bucketWasCreated.value) {
            LocalData.setBucketWasCreatedStatus();
        }

        step.value = CreateStep.Success;
        emit('created', bucketName.value);
        return;
    }

    if (edgeCredentialsForCreate.value.accessKeyId) {
        await bucketsStore.createBucketWithNoPassphrase({
            name: bucketName.value,
            enableObjectLock: enableObjectLock.value,
            enableVersioning: enableVersioning.value,
            placementName: bucketLocation.value,
        });
        if (enableObjectLock.value && defaultRetentionMode.value !== NO_MODE_SET) await setObjectLockConfig(ClientType.FOR_CREATE);
        await bucketsStore.getBuckets(1, projectID);
        analyticsStore.eventTriggered(AnalyticsEvent.BUCKET_CREATED, { project_id: projectID });
        if (!bucketWasCreated.value) {
            LocalData.setBucketWasCreatedStatus();
        }

        step.value = CreateStep.Success;
        emit('created', bucketName.value);

        return;
    }

    const now = new Date();

    if (!apiKey.value) {
        const name = `${configStore.state.config.objectBrowserKeyNamePrefix}${now.getTime()}`;
        const cleanAPIKey: AccessGrant = await agStore.createAccessGrant(name, projectID);
        bucketsStore.setApiKey(cleanAPIKey.secret);
    }

    const inOneHour = new Date(now.setHours(now.getHours() + 1));

    const macaroon = await setPermissions({
        isDownload: false,
        isUpload: true,
        isList: false,
        isDelete: false,
        isPutObjectLockConfiguration: true,
        isGetObjectLockConfiguration: true,
        notAfter: inOneHour.toISOString(),
        buckets: JSON.stringify([]),
        apiKey: apiKey.value,
    });

    const accessGrant = await generateAccess({
        apiKey: macaroon,
        passphrase: '',
    }, projectID);

    const creds: EdgeCredentials = await agStore.getEdgeCredentials(accessGrant);
    bucketsStore.setEdgeCredentialsForCreate(creds);
    await bucketsStore.createBucketWithNoPassphrase({
        name: bucketName.value,
        enableObjectLock: enableObjectLock.value,
        enableVersioning: enableVersioning.value,
        placementName: bucketLocation.value,
    });
    if (enableObjectLock.value && defaultRetentionMode.value !== NO_MODE_SET) await setObjectLockConfig(ClientType.FOR_CREATE);
    await bucketsStore.getBuckets(1, projectID);
    analyticsStore.eventTriggered(AnalyticsEvent.BUCKET_CREATED, { project_id: projectID });

    if (!bucketWasCreated.value) {
        LocalData.setBucketWasCreatedStatus();
    }

    step.value = CreateStep.Success;
    emit('created', bucketName.value);
}

function formatToGBDollars(price: string): string {
    return formatPrice(decimalShift(price, CENTS_MB_TO_DOLLARS_GB_SHIFT));
}

watchEffect(() => {
    if (enableObjectLock.value) {
        enableVersioning.value = true;
    }
});

watch(enableObjectLock, value => {
    if (!value) {
        enableVersioning.value = false;
        defaultRetentionMode.value = NO_MODE_SET;
        defaultRetentionPeriod.value = 0;
        defaultRetentionPeriodUnit.value = DefaultObjectLockPeriodUnit.DAYS;
    }
});

watch(bucketLocation, async (value) => {
    if (showNewPricingTiers.value) {
        return;
    }
    pricingForLocation.value = undefined;
    if (!value || selectedPlacement.value?.pending) return;
    isGettingPricing.value = true;
    try {
        pricingForLocation.value = await billingStore.getPriceModelForPlacement({
            placementName: value,
            projectID: project.value.id,
        });
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.CREATE_BUCKET_MODAL);
    }
    isGettingPricing.value = false;
}, { immediate: true });

watch(innerContent, newContent => {
    if (newContent) {
        withLoading(async () => {
            try {
                await bucketsStore.getAllBucketsNames(project.value.id);
            } catch (error) {
                notify.notifyError(error, AnalyticsErrorEventSource.CREATE_BUCKET_MODAL);
            }
        });

        if (!selfPlacementEnabled.value || !placementDetails.value.length) return;
        let defaultIndex = placementDetails.value.findIndex(p => p.id === project.value.placement);
        defaultIndex = defaultIndex === -1 ? 0 : defaultIndex;
        bucketLocation.value = placementDetails.value[defaultIndex].idName;
        return;
    }
    // dialog has been closed
    bucketName.value = '';
    bucketLocation.value = '';
    pricingForLocation.value = undefined;
    step.value = CreateStep.Name;
    stepNumber.value = 1;
    enableVersioning.value = false;
    enableObjectLock.value = false;
    defaultRetentionMode.value = NO_MODE_SET;
    defaultRetentionPeriod.value = 0;
    defaultRetentionPeriodUnit.value = DefaultObjectLockPeriodUnit.DAYS;
});
</script>
