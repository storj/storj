// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-modal :on-close="onClose">
        <template #content>
            <div class="modal">
                <Icon class="modal__icon" :class="{ warning: severity === 'warning', critical: severity === 'critical' }" />
                <h1 class="modal__title">{{ title }}</h1>
                <p class="modal__info">To get more {{ limitType }} limit, upgrade to a Pro Account. You will still get {{ limits.storageLimit | bytesToBase10String }} free storage and bandwidth per month, and only pay what you use beyond that.</p>
                <div class="modal__buttons">
                    <VButton
                        label="Cancel"
                        height="40px"
                        width="48%"
                        font-size="13px"
                        class="modal__buttons__button"
                        :is-white="true"
                        :on-press="onClose"
                    />
                    <VButton
                        label="Upgrade"
                        height="40px"
                        width="48%"
                        font-size="13px"
                        class="modal__buttons__button upgrade"
                        :on-press="onUpgrade"
                        :is-white-blue="true"
                    />
                </div>
            </div>
        </template>
    </v-modal>
</template>

<script setup lang="ts">

import { computed } from 'vue';

import { useStore } from '@/utils/hooks';
import { ProjectLimits } from '@/types/projects';

import VButton from '@/components/common/VButton.vue';
import VModal from '@/components/common/VModal.vue';

import Icon from '@/../static/images/project/chart.svg';

const store = useStore();

const props = defineProps<{
    severity: 'warning' | 'critical';
    title: string;
    limitType: string;
    onUpgrade: () => void;
    onClose: () => void
}>();

/**
 * Returns current limits from store.
 */
const limits = computed((): ProjectLimits => {
    return store.state.projectsModule.currentLimits;
});
</script>

<style scoped lang="scss">
.modal {
    max-width: 500px;
    padding: 32px;
    box-sizing: border-box;
    font-family: 'font_regular', sans-serif;
    text-align: left;

    &__icon {
        margin-bottom: 24px;

        &.critical {

            :deep(.icon-background) {
                fill: var(--c-red-1);
            }

            :deep(.icon-chart) {
                fill: var(--c-red-2);
            }
        }

        &.warning {

            :deep(.icon-background) {
                fill: var(--c-yellow-4);
            }

            :deep(.icon-chart) {
                fill: var(--c-yellow-5);
            }
        }
    }

    &__title {
        font-family: 'font_bold', sans-serif;
        font-size: 28px;
        line-height: 36px;
        letter-spacing: -0.02em;
        color: #000;
        margin-bottom: 8px;
    }

    &__info {
        font-family: 'font_regular', sans-serif;
        font-size: 16px;
        line-height: 24px;
        color: #000;
        margin-bottom: 16px;
    }

    &__buttons {
        display: flex;
        flex-direction: row;
        justify-content: space-between;

        &__button {
            padding: 16px;
            box-sizing: border-box;
            letter-spacing: -0.02em;

            &.upgrade {
                margin-left: 8px;
            }
        }
    }
}
</style>
