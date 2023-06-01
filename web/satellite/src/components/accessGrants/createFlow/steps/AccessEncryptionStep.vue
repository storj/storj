// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="encryption">
        <ContainerWithIcon :icon-and-title="FUNCTIONAL_CONTAINER_ICON_AND_TITLE[FunctionalContainer.EncryptionPassphrase]">
            <template #functional>
                <div class="encryption__radios">
                    <Radio
                        v-if="!isPromptForPassphrase"
                        id="currentPassphrase"
                        :checked="isSelectedOption(_PassphraseOption.UseExistingPassphrase)"
                        :on-check="() => setOption(_PassphraseOption.UseExistingPassphrase)"
                        label="Use the current passphrase"
                        info="Create this access with the same passphrase you use for this project.
                            This allows you to manage existing data you have uploaded with the same passphrase."
                    />
                    <Radio
                        v-else
                        id="myPassphrase"
                        :checked="isSelectedOption(_PassphraseOption.SetMyProjectPassphrase)"
                        :on-check="() => setOption(_PassphraseOption.SetMyProjectPassphrase)"
                        label="Enter my project passphrase"
                        info="You will enter your encryption passphrase on the next step. Make sure it's the same one
                            you use for this project. This allows you to manage existing data you have uploaded with the
                            same passphrase."
                    />
                    <div tabindex="0" class="encryption__radios__advanced" @click="toggleAdvanced" @keyup.space="toggleAdvanced">
                        <h2 class="encryption__radios__advanced__label">Advanced</h2>
                        <ChevronIcon
                            class="encryption__radios__advanced__chevron"
                            :class="{'encryption__radios__advanced__chevron--up': advancedShown}"
                        />
                    </div>
                    <Radio
                        v-show="advancedShown"
                        id="new passphrase"
                        :checked="isSelectedOption(_PassphraseOption.EnterNewPassphrase)"
                        :on-check="() => setOption(_PassphraseOption.EnterNewPassphrase)"
                        label="Enter a new passphrase"
                        info="Create this access with a new encryption passphrase that you can enter on the next step.
                            The access will not be able to manage any existing data."
                    />
                    <Radio
                        v-show="advancedShown"
                        id="generate passphrase"
                        :checked="isSelectedOption(_PassphraseOption.GenerateNewPassphrase)"
                        :on-check="() => setOption(_PassphraseOption.GenerateNewPassphrase)"
                        label="Generate 12-word passphrase"
                        info="Create this access with a new encryption passphrase that will be generated for you on
                            the next step. The access will not be able to manage any existing data."
                    />
                </div>
            </template>
            <template #info>
                <div v-if="advancedShown" class="encryption__warning-container">
                    <OrangeWarningIcon class="encryption__warning-container__icon" />
                    <div>
                        <p class="encryption__warning-container__message">
                            <b>Warning.</b> Creating a new passphrase for this access will prevent it from decrypting
                            data that has already been uploaded with the current passphrase.
                        </p>
                        <br>
                        <p class="encryption__warning-container__disclaimer">
                            Proceed only if you understand
                        </p>
                    </div>
                </div>
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
                />
            </template>
            <template #rightButton>
                <VButton
                    label="Create Access ->"
                    width="100%"
                    height="48px"
                    font-size="14px"
                    :on-press="onContinue"
                />
            </template>
        </ButtonsContainer>
    </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';

import {
    FUNCTIONAL_CONTAINER_ICON_AND_TITLE,
    FunctionalContainer,
    PassphraseOption,
} from '@/types/createAccessGrant';
import { useBucketsStore } from '@/store/modules/bucketsStore';

import ContainerWithIcon from '@/components/accessGrants/createFlow/components/ContainerWithIcon.vue';
import ButtonsContainer from '@/components/accessGrants/createFlow/components/ButtonsContainer.vue';
import Radio from '@/components/accessGrants/createFlow/components/Radio.vue';
import VButton from '@/components/common/VButton.vue';

import ChevronIcon from '@/../static/images/accessGrants/newCreateFlow/chevron.svg';
import OrangeWarningIcon from '@/../static/images/accessGrants/newCreateFlow/orangeWarning.svg';

const props = defineProps<{
    passphraseOption: PassphraseOption;
    setOption: (option: PassphraseOption) => void;
    onBack: () => void;
    onContinue: () => void;
}>();

const bucketsStore = useBucketsStore();

const advancedShown = ref<boolean>(false);

// We do this because imported enum is not directly accessible in template.
const _PassphraseOption = PassphraseOption;

/**
 * Indicates if user has to be prompt to enter project passphrase.
 */
const isPromptForPassphrase = computed((): boolean => {
    return bucketsStore.state.promptForPassphrase;
});

/**
 * Toggles advanced options visibility.
 */
function toggleAdvanced(): void {
    advancedShown.value = !advancedShown.value;
}

/**
 * Indicates if option is selected in root component.
 */
function isSelectedOption(option: PassphraseOption): boolean {
    return props.passphraseOption === option;
}
</script>

<style lang="scss" scoped>
.encryption {
    font-family: 'font_regular', sans-serif;

    &__radios {
        display: flex;
        flex-direction: column;
        row-gap: 16px;

        &__advanced {
            display: flex;
            align-items: center;

            &__label {
                font-family: 'font_bold', sans-serif;
                font-size: 14px;
                line-height: 20px;
                color: var(--c-black);
                text-align: left;
                cursor: pointer;
            }

            &__chevron {
                transition: transform 0.3s;
                margin-left: 8px;
                cursor: pointer;

                &--up {
                    transform: rotate(180deg);
                }
            }
        }
    }

    &__warning-container {
        background: var(--c-yellow-1);
        border: 1px solid var(--c-yellow-2);
        box-shadow: 0 7px 20px rgb(0 0 0 / 15%);
        border-radius: 10px;
        padding: 16px;
        display: flex;
        align-items: flex-start;
        margin-top: 16px;

        @media screen and (width <= 460px) {
            flex-direction: column;
        }

        &__icon {
            min-width: 32px;
            margin-right: 16px;

            @media screen and (width <= 460px) {
                margin: 0 0 16px;
            }
        }

        &__message {
            font-size: 14px;
            line-height: 20px;
            color: var(--c-black);
            text-align: left;
        }

        &__disclaimer {
            font-family: 'font_bold', sans-serif;
            font-size: 14px;
            line-height: 22px;
            color: var(--c-black);
            text-align: left;
        }
    }
}
</style>
