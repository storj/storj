// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="modal">
                <div class="modal__header">
                    <component :is="STEP_ICON_AND_TITLE[step].icon" />
                    <h1 class="modal__header__title">{{ STEP_ICON_AND_TITLE[step].title }}</h1>
                </div>
                <CreateNewAccessStep
                    v-if="step === CreateAccessStep.CreateNewAccess"
                    :on-select-type="selectAccessType"
                    :selected-access-types="selectedAccessTypes"
                    :name="accessName"
                    :set-name="setAccessName"
                    :on-continue="setPermissionsStep"
                />
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';

import { useRoute, useRouter } from '@/utils/hooks';
import { RouteConfig } from '@/router';
import { AccessType, CreateAccessStep, STEP_ICON_AND_TITLE } from '@/types/createAccessGrant';

import VModal from '@/components/common/VModal.vue';
import CreateNewAccessStep from '@/components/accessGrants/newCreateFlow/steps/CreateNewAccessStep.vue';

const router = useRouter();
const route = useRoute();

const step = ref<CreateAccessStep>(CreateAccessStep.CreateNewAccess);
const selectedAccessTypes = ref<AccessType[]>([]);
const accessName = ref<string>('');

/**
 * Selects access type.
 */
function selectAccessType(type: AccessType) {
    // "access grant" and "s3 credentials" can be selected at the same time,
    // but "API key" cannot be selected if either "access grant" or "s3 credentials" is selected.
    switch (type) {
    case AccessType.AccessGrant:
        // Unselect API key if was selected.
        unselectAPIKeyAccessType();

        // Unselect Access grant if was selected.
        if (selectedAccessTypes.value.includes(AccessType.AccessGrant)) {
            selectedAccessTypes.value = selectedAccessTypes.value.filter(t => t !== AccessType.AccessGrant);
            return;
        }

        // Select Access grant.
        selectedAccessTypes.value.push(type);
        break;
    case AccessType.S3:
        // Unselect API key if was selected.
        unselectAPIKeyAccessType();

        // Unselect S3 if was selected.
        if (selectedAccessTypes.value.includes(AccessType.S3)) {
            selectedAccessTypes.value = selectedAccessTypes.value.filter(t => t !== AccessType.S3);
            return;
        }

        // Select S3.
        selectedAccessTypes.value.push(type);
        break;
    case AccessType.APIKey:
        // Unselect Access grant and S3 if were selected.
        if (selectedAccessTypes.value.includes(AccessType.AccessGrant) || selectedAccessTypes.value.includes(AccessType.S3)) {
            selectedAccessTypes.value = selectedAccessTypes.value.filter(t => t === AccessType.APIKey);
        }

        // Unselect API key if was selected.
        if (selectedAccessTypes.value.includes(AccessType.APIKey)) {
            selectedAccessTypes.value = selectedAccessTypes.value.filter(t => t !== AccessType.APIKey);
            return;
        }

        // Select API key.
        selectedAccessTypes.value.push(type);
    }
}

/**
 * Unselects API key access type.
 */
function unselectAPIKeyAccessType(): void {
    if (selectedAccessTypes.value.includes(AccessType.APIKey)) {
        selectedAccessTypes.value = selectedAccessTypes.value.filter(t => t !== AccessType.APIKey);
    }
}

/**
 * Sets access grant name from input field.
 * @param value
 */
function setAccessName(value: string): void {
    accessName.value = value;
}

/**
 * Sets current step to be 'Choose permission'.
 */
function setPermissionsStep(): void {
    step.value = CreateAccessStep.ChoosePermission;
}

/**
 * Closes create access grant flow.
 */
function closeModal(): void {
    router.push(RouteConfig.AccessGrants.path);
}

onMounted(() => {
    if (route.params?.accessType) {
        selectedAccessTypes.value.push(route.params?.accessType as AccessType);
    }
});
</script>

<style scoped lang="scss">
.modal {
    width: 346px;
    padding: 32px;
    display: flex;
    flex-direction: column;

    &__header {
        display: flex;
        align-items: center;
        padding-bottom: 16px;
        border-bottom: 1px solid #ebeef1;

        &__title {
            margin-left: 16px;
            font-family: 'font_bold', sans-serif;
            font-size: 24px;
            line-height: 31px;
            letter-spacing: -0.02em;
            color: #000;
        }
    }
}
</style>
