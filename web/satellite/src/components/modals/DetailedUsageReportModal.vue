// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="modal">
                <div class="modal__header">
                    <h1 class="modal__header__title">Get Detailed Usage Report</h1>
                </div>
                <p class="modal__label">Select date range to generate your report:</p>
                <div class="modal__options">
                    <VButton
                        label="Past Month"
                        width="100%"
                        height="32px"
                        font-size="14px"
                        border-radius="10px"
                        :on-press="setPastMonth"
                        :is-transparent="activeOption !== Options.PastMonth"
                    />
                    <VButton
                        label="Past Year"
                        width="100%"
                        height="32px"
                        font-size="14px"
                        border-radius="10px"
                        :on-press="setPastYear"
                        :is-transparent="activeOption !== Options.PastYear"
                    />
                    <VButton
                        label="Choose Dates"
                        width="100%"
                        height="32px"
                        font-size="14px"
                        border-radius="10px"
                        :on-press="setChooseDates"
                        :is-transparent="activeOption !== Options.ChooseDates"
                    />
                </div>
                <v-date-range-picker
                    :is-open="activeOption === Options.ChooseDates"
                    :on-date-pick="setFromPicker"
                />
                <div class="modal__button-container">
                    <VButton
                        label="Cancel"
                        width="100%"
                        height="48px"
                        font-size="14px"
                        border-radius="10px"
                        :on-press="closeModal"
                        :is-transparent="true"
                    />
                    <VButton
                        label="Download"
                        width="100%"
                        height="48px"
                        font-size="14px"
                        border-radius="10px"
                        :on-press="onDownload"
                    />
                </div>
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { onBeforeMount, onBeforeUnmount, ref } from 'vue';

import { useNotify } from '@/utils/hooks';
import { useAppStore } from '@/store/modules/appStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { Download } from '@/utils/download';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

import VButton from '@/components/common/VButton.vue';
import VModal from '@/components/common/VModal.vue';
import VDateRangePicker from '@/components/common/VDateRangePicker.vue';

enum Options {
    PastMonth = 0,
    PastYear,
    ChooseDates,
}

const appStore = useAppStore();
const projectsStore = useProjectsStore();

const notify = useNotify();

const activeOption = ref<Options>(Options.PastMonth);
const since = ref<Date>();
const before = ref<Date>();

/**
 * Starts report downloading.
 */
async function onDownload(): Promise<void> {
    if (!(since.value && before.value)) {
        notify.error('Please select date range', AnalyticsErrorEventSource.DETAILED_USAGE_REPORT_MODAL);
        return;
    }

    try {
        const link = projectsStore.getUsageReportLink(since.value, before.value, appStore.state.usageReportProjectID);
        Download.fileByLink(link);
        notify.success('Usage report download started successfully.');
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.DETAILED_USAGE_REPORT_MODAL);
    }
}

/**
 * Sets past month as active option.
 */
function setPastMonth(): void {
    const now = new Date();

    since.value = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() - 1, now.getUTCDate(), now.getUTCHours(), now.getUTCMinutes()));
    before.value = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate(), now.getUTCHours(), now.getUTCMinutes()));
    activeOption.value = Options.PastMonth;
}

/**
 * Sets past year as active option.
 */
function setPastYear(): void {
    const now = new Date();

    since.value = new Date(Date.UTC(now.getUTCFullYear() - 1, now.getUTCMonth(), now.getUTCDate(), now.getUTCHours(), now.getUTCMinutes()));
    before.value = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate(), now.getUTCHours(), now.getUTCMinutes()));
    activeOption.value = Options.PastYear;
}

/**
 * Sets custom date range as active option.
 */
function setChooseDates(): void {
    since.value = undefined;
    before.value = undefined;
    activeOption.value = Options.ChooseDates;
}

/**
 * Sets date range from picker.
 */
function setFromPicker(dateRange: Date[]): void {
    const start = dateRange[0];
    const end = dateRange[1];

    since.value = new Date(Date.UTC(start.getFullYear(), start.getMonth(), start.getDate(), start.getHours(), start.getMinutes()));
    before.value = new Date(Date.UTC(end.getFullYear(), end.getMonth(), end.getDate(), end.getHours(), end.getMinutes()));
}

/**
 * Closes modal.
 */
function closeModal(): void {
    appStore.removeActiveModal();
}

onBeforeMount(() => {
    setPastMonth();
});

onBeforeUnmount(() => {
    appStore.setUsageReportProjectID('');
});
</script>

<style scoped lang="scss">
.modal {
    padding: 32px 16px;
    box-sizing: border-box;
    font-family: 'font_regular', sans-serif;
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    width: 500px;

    @media screen and (width <= 550px) {
        width: unset;
    }

    &__header {
        display: flex;
        align-items: center;
        padding-bottom: 16px;
        margin-bottom: 32px;
        border-bottom: 1px solid var(--c-grey-2);

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 24px;
            line-height: 31px;
            color: var(--c-grey-8);
            text-align: left;
        }
    }

    &__options {
        margin: 10px 0 16px;
        width: 100%;
        display: flex;
        align-items: center;
        column-gap: 8px;
    }

    &__button-container {
        width: 100%;
        display: flex;
        align-items: center;
        justify-content: space-between;
        margin-top: 16px;
        column-gap: 20px;

        @media screen and (width <= 550px) {
            margin-top: 20px;
            column-gap: unset;
            row-gap: 8px;
            flex-direction: column-reverse;
        }
    }
}
</style>
