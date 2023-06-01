// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="confirm">
        <ContainerWithIcon :icon-and-title="FUNCTIONAL_CONTAINER_ICON_AND_TITLE[FunctionalContainer.Type]">
            <template #functional>
                <p class="confirm__label">{{ accessTypes.join(', ') }}</p>
            </template>
        </ContainerWithIcon>
        <ContainerWithIcon :icon-and-title="FUNCTIONAL_CONTAINER_ICON_AND_TITLE[FunctionalContainer.Name]">
            <template #functional>
                <p class="confirm__label">{{ name }}</p>
            </template>
        </ContainerWithIcon>
        <ContainerWithIcon :icon-and-title="FUNCTIONAL_CONTAINER_ICON_AND_TITLE[FunctionalContainer.Permissions]">
            <template #functional>
                <p class="confirm__label">{{ selectedPermissions.length === 4 ? 'All' : selectedPermissions.join(', ') }}</p>
            </template>
        </ContainerWithIcon>
        <ContainerWithIcon :icon-and-title="FUNCTIONAL_CONTAINER_ICON_AND_TITLE[FunctionalContainer.Buckets]">
            <template #functional>
                <p class="confirm__label">{{ selectedBuckets.length === 0 ? 'All' : selectedBuckets.join(', ') }}</p>
            </template>
        </ContainerWithIcon>
        <ContainerWithIcon :icon-and-title="FUNCTIONAL_CONTAINER_ICON_AND_TITLE[FunctionalContainer.EndDate]">
            <template #functional>
                <p class="confirm__label">{{ notAfterLabel }}</p>
            </template>
        </ContainerWithIcon>
        <ButtonsContainer>
            <template #leftButton>
                <VButton
                    label="Back"
                    width="100%"
                    height="48px"
                    font-size="14px"
                    :on-press="onBack"
                    :is-white="true"
                    :is-disabled="isLoading"
                />
            </template>
            <template #rightButton>
                <VButton
                    label="Confirm"
                    width="100%"
                    height="48px"
                    font-size="14px"
                    :on-press="onContinue"
                    :is-disabled="isLoading"
                />
            </template>
        </ButtonsContainer>
    </div>
</template>

<script setup lang="ts">
import {
    AccessType,
    FUNCTIONAL_CONTAINER_ICON_AND_TITLE,
    FunctionalContainer,
    Permission,
} from '@/types/createAccessGrant';

import ContainerWithIcon from '@/components/accessGrants/createFlow/components/ContainerWithIcon.vue';
import ButtonsContainer from '@/components/accessGrants/createFlow/components/ButtonsContainer.vue';
import VButton from '@/components/common/VButton.vue';

const props = defineProps<{
    name: string;
    accessTypes: AccessType[];
    selectedPermissions: Permission[];
    selectedBuckets: string[];
    notAfterLabel: string;
    onBack: () => void;
    onContinue: () => void;
    isLoading: boolean;
}>();
</script>

<style lang="scss" scoped>
.confirm {
    font-family: 'font_regular', sans-serif;

    &__label {
        font-size: 14px;
        line-height: 20px;
        color: var(--c-black);
        text-align: left;
        word-break: break-word;
    }
}
</style>
