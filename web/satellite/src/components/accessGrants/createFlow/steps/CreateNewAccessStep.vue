// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="create">
        <ContainerWithIcon :icon-and-title="FUNCTIONAL_CONTAINER_ICON_AND_TITLE[FunctionalContainer.Type]">
            <template #functional>
                <div class="create__toggles">
                    <Toggle
                        :checked="selectedAccessTypes.includes(AccessType.AccessGrant)"
                        label="Access Grant"
                        :on-check="() => onSelectType(AccessType.AccessGrant)"
                    >
                        <template #infoMessage>
                            <p class="create__toggles__info">
                                Keys to upload, delete, and view your project's data.
                                <a
                                    tabindex="0"
                                    class="create__toggles__info__link"
                                    href="https://docs.storj.io/dcs/concepts/access/access-grants"
                                    target="_blank"
                                    rel="noopener noreferrer"
                                    @click="() => trackPageVisit('https://docs.storj.io/dcs/concepts/access/access-grants')"
                                >Learn More</a>
                            </p>
                        </template>
                    </Toggle>
                    <Toggle
                        :checked="selectedAccessTypes.includes(AccessType.S3)"
                        label="S3 Credentials"
                        :on-check="() => onSelectType(AccessType.S3)"
                    >
                        <template #infoMessage>
                            <p class="create__toggles__info">
                                Generates access key, secret key, and endpoint to use in your S3-supporting application.
                                <a
                                    tabindex="0"
                                    class="create__toggles__info__link"
                                    href="https://docs.storj.io/dcs/api-reference/s3-compatible-gateway"
                                    target="_blank"
                                    rel="noopener noreferrer"
                                    @click="() => trackPageVisit('https://docs.storj.io/dcs/api-reference/s3-compatible-gateway')"
                                >Learn More</a>
                            </p>
                        </template>
                    </Toggle>
                    <Toggle
                        :checked="selectedAccessTypes.includes(AccessType.APIKey)"
                        label="CLI Access"
                        :on-check="() => onSelectType(AccessType.APIKey)"
                    >
                        <template #infoMessage>
                            <p class="create__toggles__info">
                                Creates access grant to run in the command line.
                                <a
                                    tabindex="0"
                                    class="create__toggles__info__link"
                                    href="https://docs.storj.io/dcs/getting-started/quickstart-uplink-cli/generate-access-grants-and-tokens/generate-a-token"
                                    target="_blank"
                                    rel="noopener noreferrer"
                                    @click="() => trackPageVisit('https://docs.storj.io/dcs/getting-started/quickstart-uplink-cli/generate-access-grants-and-tokens/generate-a-token')"
                                >Learn More</a>
                            </p>
                        </template>
                    </Toggle>
                </div>
            </template>
        </ContainerWithIcon>
        <ContainerWithIcon :icon-and-title="FUNCTIONAL_CONTAINER_ICON_AND_TITLE[FunctionalContainer.Name]">
            <template #functional>
                <VInput
                    class="create__input"
                    placeholder="Input Access Name"
                    :init-value="name"
                    @setData="setName"
                />
            </template>
        </ContainerWithIcon>
        <ButtonsContainer>
            <template #leftButton>
                <LinkButton
                    label="Learn more"
                    link="https://docs.storj.io/dcs/concepts/access/access-grants/api-key"
                    :with-icon="true"
                />
            </template>
            <template #rightButton>
                <VButton
                    label="Continue ->"
                    width="100%"
                    height="48px"
                    font-size="14px"
                    :on-press="onContinue"
                    :is-disabled="isButtonDisabled"
                />
            </template>
        </ButtonsContainer>
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { AccessType, FUNCTIONAL_CONTAINER_ICON_AND_TITLE, FunctionalContainer } from '@/types/createAccessGrant';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import ContainerWithIcon from '@/components/accessGrants/createFlow/components/ContainerWithIcon.vue';
import ButtonsContainer from '@/components/accessGrants/createFlow/components/ButtonsContainer.vue';
import LinkButton from '@/components/accessGrants/createFlow/components/LinkButton.vue';
import Toggle from '@/components/accessGrants/createFlow/components/Toggle.vue';
import VInput from '@/components/common/VInput.vue';
import VButton from '@/components/common/VButton.vue';

const props = defineProps<{
    name: string;
    setName: (value: string) => void;
    selectedAccessTypes: AccessType[];
    onSelectType: (type: AccessType) => void;
    onContinue: () => void;
}>();

const analyticsStore = useAnalyticsStore();

/**
 * Indicates if button should be disabled.
 */
const isButtonDisabled = computed((): boolean => {
    return !props.name || !props.selectedAccessTypes.length;
});

/**
 * Sends "trackPageVisit" event to segment.
 */
function trackPageVisit(link: string): void {
    analyticsStore.pageVisit(link);
}
</script>

<style lang="scss" scoped>
.create {
    font-family: 'font_regular', sans-serif;

    &__toggles {
        display: flex;
        flex-direction: column;
        row-gap: 16px;

        &__info {
            color: var(--c-white);

            &__link {
                color: var(--c-white);
                text-decoration: underline !important;
                text-underline-position: under;

                &:visited {
                    color: var(--c-white);
                }

                &:focus {
                    outline: 2px solid #fff;
                }
            }
        }
    }

    &__input {
        margin-top: 0;
    }
}

:deep(.label-container) {
    display: none;
}
</style>
