// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="usage-charges-item-container">
        <div class="usage-charges-item-container__summary" @click="toggleDetailedInfo">
            <div class="usage-charges-item-container__summary__name-container">
                <ChargesExpandIcon v-if="isDetailedInfoShown" />
                <ChargesHideIcon v-else />
                <span>{{ projectName }}</span>
            </div>
            <span class="usage-charges-item-container__summary__amount">
                Estimated Total {{ item.summary() | centsToDollars }}
            </span>
        </div>
        <div v-if="isDetailedInfoShown" class="usage-charges-item-container__detailed-info-container">
            <div class="usage-charges-item-container__detailed-info-container__info-header">
                <span class="resource-header">RESOURCE</span>
                <span class="period-header">PERIOD</span>
                <span class="usage-header">USAGE</span>
                <span class="cost-header">COST</span>
            </div>
            <div class="usage-charges-item-container__detailed-info-container__content-area">
                <div class="usage-charges-item-container__detailed-info-container__content-area__resource-container">
                    <p>Storage (${{ storagePrice }} per Gigabyte-Month)</p>
                    <p>Egress (${{ egressPrice }} per GB)</p>
                    <p>Segments (${{ segmentPrice }} per Segment-Month)</p>
                </div>
                <div class="usage-charges-item-container__detailed-info-container__content-area__period-container">
                    <p>{{ period }}</p>
                    <p>{{ period }}</p>
                    <p>{{ period }}</p>
                </div>
                <div class="usage-charges-item-container__detailed-info-container__content-area__usage-container">
                    <p>{{ storageFormatted }} Gigabyte-month</p>
                    <p>{{ egressAmountAndDimension }}</p>
                    <p>{{ segmentCountFormatted }} Segment-month</p>
                </div>
                <div class="usage-charges-item-container__detailed-info-container__content-area__cost-container">
                    <p class="price">{{ item.storagePrice | centsToDollars }}</p>
                    <p class="price">{{ item.egressPrice | centsToDollars }}</p>
                    <p class="price">{{ item.segmentPrice | centsToDollars }}</p>
                </div>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';

import { ProjectUsageAndCharges, ProjectUsagePriceModel } from '@/types/payments';
import { Project } from '@/types/projects';
import { Size } from '@/utils/bytesSize';
import { SHORT_MONTHS_NAMES } from '@/utils/constants/date';
import { decimalShift } from '@/utils/strings';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useStore } from '@/utils/hooks';

import ChargesHideIcon from '@/../static/images/account/billing/chargesHide.svg';
import ChargesExpandIcon from '@/../static/images/account/billing/chargesExpand.svg';

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

/**
 * HOURS_IN_MONTH constant shows amount of hours in 30-day month.
 */
const HOURS_IN_MONTH = 720;

/**
 * CENTS_MB_TO_DOLLARS_GB_SHIFT constant represents how many places to the left
 * a decimal point must be shifted to convert from cents/MB to dollars/GB.
 */
const CENTS_MB_TO_DOLLARS_GB_SHIFT = -1;

const props = withDefaults(defineProps<{
    /**
     * item represents usage and charges of current project by period.
     */
    item?: ProjectUsageAndCharges;
}>(), {
    item: () => new ProjectUsageAndCharges(),
});

const store = useStore();

/**
 * isDetailedInfoShown indicates if area with detailed information about project charges is expanded.
 */
const isDetailedInfoShown = ref<boolean>(false);

/**
 * projectName returns project name.
 */
const projectName = computed((): string => {
    const projects: Project[] = store.state.projectsModule.projects;
    const project: Project = projects.find(project => project.id === props.item.projectId);

    return project?.name || '';
});

/**
 * Returns string of date range.
 */
const period = computed((): string => {
    const since = `${SHORT_MONTHS_NAMES[props.item.since.getUTCMonth()]} ${props.item.since.getUTCDate()}`;
    const before = `${SHORT_MONTHS_NAMES[props.item.before.getUTCMonth()]} ${props.item.before.getUTCDate()}`;

    return `${since} - ${before}`;
});

/**
 * Returns project usage price model from store.
 */
const priceModel = computed((): ProjectUsagePriceModel => {
    return store.state.paymentsModule.usagePriceModel;
});

/**
 * Returns formatted egress depending on amount of bytes.
 */
const egressFormatted = computed((): Size => {
    return new Size(props.item.egress, 2);
});

/**
 * Returns formatted storage used in GB x month dimension.
 */
const storageFormatted = computed((): string => {
    const bytesInGB = 1000000000;

    return (props.item.storage / HOURS_IN_MONTH / bytesInGB).toFixed(2);
});

/**
 * Returns formatted segment count in segment x month dimension.
 */
const segmentCountFormatted = computed((): string => {
    return (props.item.segmentCount / HOURS_IN_MONTH).toFixed(2);
});

/**
 * Returns storage price per GB.
 */
const storagePrice = computed((): string => {
    return decimalShift(priceModel.value.storageMBMonthCents, CENTS_MB_TO_DOLLARS_GB_SHIFT);
});

/**
 * Returns egress price per GB.
 */
const egressPrice = computed((): string => {
    return decimalShift(priceModel.value.egressMBCents, CENTS_MB_TO_DOLLARS_GB_SHIFT);
});

/**
 * Returns segment price.
 */
const segmentPrice = computed((): string => {
    return decimalShift(priceModel.value.segmentMonthCents, 2);
});

/**
 * Returns string of egress amount and dimension.
 */
const egressAmountAndDimension = computed((): string => {
    return `${egressFormatted.value.formattedBytes} ${egressFormatted.value.label}`;
});

/**
 * toggleDetailedInfo expands an area with detailed information about project charges.
 */
function toggleDetailedInfo(): void {
    analytics.eventTriggered(AnalyticsEvent.USAGE_DETAILED_INFO_CLICKED);
    isDetailedInfoShown.value = !isDetailedInfoShown.value;
}
</script>

<style scoped lang="scss">
    p {
        margin: 0;
    }

    .usage-charges-item-container {
        font-size: 16px;
        line-height: 21px;
        padding: 30px 0;
        font-family: 'font_regular', sans-serif;

        &__summary {
            display: flex;
            justify-content: space-between;
            align-items: center;
            cursor: pointer;

            &__name-container {
                display: flex;
                align-items: center;

                &__expand-image,
                &__hide-image {
                    width: 14px;
                    height: 14px;
                    margin-right: 12px;
                }

                &__expand-image {
                    height: 8px;
                }
            }

            &__amount {
                font-size: 16px;
                line-height: 21px;
                text-align: right;
                color: #354049;
            }
        }

        &__detailed-info-container {
            display: flex;
            flex-direction: column;
            align-items: flex-start;
            justify-content: space-between;
            padding: 16px 0 0 26px;

            &__info-header {
                display: flex;
                align-items: center;
                justify-content: space-between;
                font-size: 14px;
                line-height: 19px;
                color: #adadad;
                height: 25px;
                width: 100%;
            }

            &__content-area {
                width: 100%;
                padding: 10px 0 0;
                display: flex;
                align-items: center;
                justify-content: space-between;

                &__resource-container,
                &__period-container,
                &__cost-container,
                &__usage-container {
                    width: 20%;
                    font-size: 14px;
                    line-height: 19px;
                    color: #354049;

                    :nth-child(1),
                    :nth-child(2) {
                        margin-bottom: 3px;
                    }
                }

                &__resource-container {
                    width: 40%;
                }
            }

            &__link-container {
                width: 100%;
                display: flex;
                justify-content: flex-end;
                align-items: center;
                margin-top: 25px;

                &__link {
                    font-size: 13px;
                    line-height: 19px;
                    color: #2683ff;
                    cursor: pointer;
                }
            }
        }
    }

    .resource-header {
        width: 40%;
    }

    .cost-header,
    .period-header,
    .usage-header {
        width: 20%;
    }

    .cost-header,
    .price {
        text-align: right;
    }
</style>
