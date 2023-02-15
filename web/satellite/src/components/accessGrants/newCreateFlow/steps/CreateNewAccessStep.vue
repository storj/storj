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
                                    class="create__toggles__info__link"
                                    href="https://docs.storj.io/dcs/concepts/access/access-grants"
                                    target="_blank"
                                    rel="noopener noreferrer"
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
                                    class="create__toggles__info__link"
                                    href="https://docs.storj.io/dcs/api-reference/s3-compatible-gateway"
                                    target="_blank"
                                    rel="noopener noreferrer"
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
                                    class="create__toggles__info__link"
                                    href="https://docs.storj.io/dcs/getting-started/quickstart-uplink-cli/generate-access-grants-and-tokens/generate-a-token"
                                    target="_blank"
                                    rel="noopener noreferrer"
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
                <a
                    class="create__button-link"
                    href="https://docs.storj.io/dcs/concepts/access/access-grants/api-key"
                    target="_blank"
                    rel="noopener noreferrer"
                >
                    <LearnIcon />
                    <p class="create__button-link__label">
                        Learn more
                    </p>
                </a>
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

import ContainerWithIcon from '@/components/accessGrants/newCreateFlow/components/ContainerWithIcon.vue';
import ButtonsContainer from '@/components/accessGrants/newCreateFlow/components/ButtonsContainer.vue';
import Toggle from '@/components/accessGrants/newCreateFlow/components/Toggle.vue';
import VInput from '@/components/common/VInput.vue';
import VButton from '@/components/common/VButton.vue';

import LearnIcon from '@/../static/images/accessGrants/newCreateFlow/learn.svg';

const props = defineProps<{
    name: string;
    setName: (value: string) => void;
    selectedAccessTypes: AccessType[];
    onSelectType: (type: AccessType) => void;
    onContinue: () => void;
}>();

/**
 * Indicates if button should be disabled.
 */
const isButtonDisabled = computed((): boolean => {
    return !props.name || !props.selectedAccessTypes.length;
});
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
            }
        }
    }

    &__input {
        margin-top: 0;
    }

    &__button-link {
        display: flex;
        align-items: center;
        justify-content: center;
        width: 100%;
        height: 48px;
        background: var(--c-white);
        border: 1px solid var(--c-grey-3);
        box-shadow: 0 0 3px rgb(0 0 0 / 8%);
        border-radius: 8px;

        &__label {
            font-family: 'font_medium', sans-serif;
            font-size: 14px;
            line-height: 24px;
            letter-spacing: -0.02em;
            color: var(--c-grey-6);
            margin-left: 8px;
        }

        &:hover {
            border-color: var(--c-light-blue-6);
            background-color: var(--c-light-blue-6);

            p {
                color: var(--c-white);
            }

            :deep(svg path) {
                fill: var(--c-white);
            }
        }
    }
}

:deep(.label-container) {
    display: none;
}
</style>
