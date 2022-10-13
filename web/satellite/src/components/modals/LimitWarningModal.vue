// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-modal :on-close="onClose">
        <template #content>
            <div class="modal">
                <Icon class="modal__icon" :class="{ warning: severity === 'warning', critical: severity === 'critical' }" />
                <h1 class="modal__title">{{ title }}</h1>
                <p class="modal__info">To get more storage and bandwidth, upgrade to a Pro Account. You will still get 150GB free storage and bandwidth per month, and only pay what you use beyond that.</p>
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
                        :is-green-white="true"
                    />
                </div>
            </div>
        </template>
    </v-modal>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import VButton from '@/components/common/VButton.vue';
import VModal from '@/components/common/VModal.vue';

import Icon from '@/../static/images/project/chart.svg';

// @vue/component
@Component({
    components: {
        VButton,
        VModal,
        Icon,
    },
})
export default class LimitWarningModal extends Vue {
    @Prop({ default: 'warning' })
    private readonly severity: 'warning' | 'critical';
    @Prop({ default: '' })
    private readonly title: string;
    @Prop({ default: () => {} })
    private readonly onUpgrade: () => Promise<void>;
    @Prop({ default: () => {} })
    private readonly onClose: () => void;
}
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
                fill: #ffb0b0;
            }

            :deep(.icon-chart) {
                fill: #ff1313;
            }
        }

        &.warning {

            :deep(.icon-background) {
                fill: #ffa800;
            }

            :deep(.icon-chart) {
                fill: #ff8a00;
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

:deep(.whiteCheck) {
    display: none;
}
</style>
