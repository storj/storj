// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="usage-charges-item-container">
        <div class="usage-charges-item-container__summary" @click="toggleDetailedInfo">
            <div class="usage-charges-item-container__summary__name-container">
                <GreyChevron :class="`chevron-${isDetailedInfoShown?'down':'up'}`" />
                <span class="usage-charges-item-container__summary__name-container__name">{{ projectName }}</span>
            </div>
            <span class="usage-charges-item-container__summary__text">
                Estimated Total &nbsp;
                <span
                    class="usage-charges-item-container__summary__amount"
                >{{ projectCharges.getProjectPrice(projectId) | centsToDollars }}
                </span>
            </span>
        </div>
        <template v-if="isDetailedInfoShown">
            <div
                v-for="[partner, charge] in partnerCharges"
                :key="partner"
                class="usage-charges-item-container__detailed-info-container"
            >
                <p v-if="partnerCharges.length > 1 || partner" class="usage-charges-item-container__detailed-info-container__partner">
                    {{ partner || 'Standard Usage' }}
                </p>
                <div class="usage-charges-item-container__detailed-info-container__info-header">
                    <span class="resource-header">RESOURCE</span>
                    <span class="period-header">PERIOD</span>
                    <span class="usage-header">USAGE</span>
                    <span class="cost-header">COST</span>
                </div>
                <div class="usage-charges-item-container__detailed-info-container__content-area">
                    <div class="usage-charges-item-container__detailed-info-container__content-area__resource-container">
                        <p>Storage <span class="price-per-month">({{ getStoragePrice(partner) }} per Gigabyte-Month)</span></p>
                        <p>Egress <span class="price-per-month">({{ getEgressPrice(partner) }} per GB)</span></p>
                        <p>Segments <span class="price-per-month">({{ getSegmentPrice(partner) }} per Segment-Month)</span></p>
                    </div>
                    <div class="usage-charges-item-container__detailed-info-container__content-area__period-container">
                        <p v-for="i in 3" :key="i">{{ getPeriod(charge) }}</p>
                    </div>
                    <div class="usage-charges-item-container__detailed-info-container__content-area__usage-container">
                        <p>{{ getStorageFormatted(charge) }} Gigabyte-month</p>
                        <p>{{ getEgressAmountAndDimension(charge) }}</p>
                        <p>{{ getSegmentCountFormatted(charge) }} Segment-month</p>
                    </div>
                    <div class="usage-charges-item-container__detailed-info-container__content-area__cost-container">
                        <p class="price">{{ charge.storagePrice | centsToDollars }}</p>
                        <p class="price">{{ charge.egressPrice | centsToDollars }}</p>
                        <p class="price">{{ charge.segmentPrice | centsToDollars }}</p>
                    </div>
                </div>
            </div>
        </template>
    </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';

import { ProjectCharge, ProjectCharges, ProjectUsagePriceModel } from '@/types/payments';
import { Project } from '@/types/projects';
import { Size } from '@/utils/bytesSize';
import { SHORT_MONTHS_NAMES } from '@/utils/constants/date';
import { decimalShift, formatPrice, CENTS_MB_TO_DOLLARS_GB_SHIFT } from '@/utils/strings';
import { useBillingStore } from '@/store/modules/billingStore';
import { useProjectsStore } from '@/store/modules/projectsStore';

import GreyChevron from '@/../static/images/common/greyChevron.svg';

/**
 * HOURS_IN_MONTH constant shows amount of hours in 30-day month.
 */
const HOURS_IN_MONTH = 720;

const props = withDefaults(defineProps<{
    /**
     * The ID of the project for which to show the usage and charge information.
     */
    projectId?: string;
}>(), {
    projectId: '',
});

const billingStore = useBillingStore();
const projectsStore = useProjectsStore();

/**
 * isDetailedInfoShown indicates if area with detailed information about project charges is expanded.
 */
const isDetailedInfoShown = ref<boolean>(false);

/**
 * An array of tuples containing the partner name and usage charge for the specified project ID.
 */
const partnerCharges = computed((): [partner: string, charge: ProjectCharge][] => {
    const arr = billingStore.state.projectCharges.toArray();
    arr.sort(([partner1], [partner2]) => partner1.localeCompare(partner2));
    const tuple = arr.find(tuple => tuple[0] === props.projectId);
    return tuple ? tuple[1] : [];
});

/**
 * projectName returns project name.
 */
const projectName = computed((): string => {
    const projects: Project[] = projectsStore.state.projects;
    const project: Project | undefined = projects.find(project => project.id === props.projectId);

    return project?.name || '';
});

/**
 * Returns project usage price model from store.
 */
const projectCharges = computed((): ProjectCharges => {
    return billingStore.state.projectCharges as ProjectCharges;
});

/**
 * Returns project usage price model from store.
 */
function getPriceModel(partner: string): ProjectUsagePriceModel {
    return projectCharges.value.getUsagePriceModel(partner) || billingStore.state.usagePriceModel;
}

/**
 * Returns string of date range.
 */
function getPeriod(charge: ProjectCharge): string {
    const since = `${SHORT_MONTHS_NAMES[charge.since.getUTCMonth()]} ${charge.since.getUTCDate()}`;
    const before = `${SHORT_MONTHS_NAMES[charge.before.getUTCMonth()]} ${charge.before.getUTCDate()}`;

    return `${since} - ${before}`;
}

/**
 * Returns formatted egress depending on amount of bytes.
 */
function egressFormatted(charge: ProjectCharge): Size {
    return new Size(charge.egress, 2);
}

/**
 * Returns formatted storage used in GB x month dimension.
 */
function getStorageFormatted(charge: ProjectCharge): string {
    const bytesInGB = 1000000000;

    return (charge.storage / HOURS_IN_MONTH / bytesInGB).toFixed(2);
}

/**
 * Returns formatted segment count in segment x month dimension.
 */
function getSegmentCountFormatted(charge: ProjectCharge): string {
    return (charge.segmentCount / HOURS_IN_MONTH).toFixed(2);
}

/**
 * Returns storage price per GB.
 */
function getStoragePrice(partner: string): string {
    return formatPrice(decimalShift(getPriceModel(partner).storageMBMonthCents, CENTS_MB_TO_DOLLARS_GB_SHIFT));
}

/**
 * Returns egress price per GB.
 */
function getEgressPrice(partner: string): string {
    return formatPrice(decimalShift(getPriceModel(partner).egressMBCents, CENTS_MB_TO_DOLLARS_GB_SHIFT));
}

/**
 * Returns segment price.
 */
function getSegmentPrice(partner: string): string {
    return formatPrice(decimalShift(getPriceModel(partner).segmentMonthCents, 2));
}

/**
 * Returns string of egress amount and dimension.
 */
function getEgressAmountAndDimension(charge: ProjectCharge): string {
    const egress = egressFormatted(charge);
    return `${egress.formattedBytes} ${egress.label}`;
}

/**
 * toggleDetailedInfo expands an area with detailed information about project charges.
 */
function toggleDetailedInfo(): void {
    isDetailedInfoShown.value = !isDetailedInfoShown.value;
}
</script>

<style scoped lang="scss">
    p {
        margin: 0;
    }

    .divider {
        margin-top: 15px;
        background-color: var(--c-grey-2);
        width: calc(100% + 40px);
        height: 1px;
        align-self: center;
    }

    .chevron-up {
        transition: 200ms;
        transform: rotate(-90deg);
    }

    .chevron-down {
        transition: 200ms;
    }

    .usage-charges-item-container {
        color: var(--c-black);
        font-size: 16px;
        line-height: 21px;
        margin-top: 10px;
        font-family: 'font_regular', sans-serif;
        background-color: var(--c-white);
        border-radius: 8px;
        box-shadow: 0 0 20px rgb(0 0 0 / 4%);

        &__summary {
            display: flex;
            justify-content: space-between;
            align-items: center;
            flex-wrap: wrap;
            padding: 20px;
            cursor: pointer;

            &__name-container {
                display: flex;
                align-items: center;

                &__name {
                    margin-left: 10px;
                }

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

            &__text {
                font-size: 17px;
                line-height: 21px;
                text-align: right;
                display: flex;
                align-items: center;
            }

            &__amount {
                font-family: 'font_bold', sans-serif;
                font-size: 24px;
                line-height: 31px;
                text-align: right;
            }
        }

        &__detailed-info-container {
            display: flex;
            flex-direction: column;
            align-items: flex-start;
            justify-content: space-between;
            padding: 24px 20px;
            border-top: 1px solid var(--c-grey-2);

            &__partner {
                font-size: 17px;
                margin-bottom: 16px;
            }

            &__info-header {
                display: flex;
                align-items: center;
                justify-content: space-between;
                font-size: 12px;
                line-height: 19px;
                color: var(--c-grey-6);
                font-weight: 600;
                height: 25px;
                width: 100%;
            }

            &__content-area {
                width: 100%;
                padding: 5px 0 0;
                display: flex;
                align-items: center;
                justify-content: space-between;

                &__resource-container,
                &__period-container,
                &__cost-container,
                &__usage-container {
                    width: 20%;
                    font-size: 16px;
                    line-height: 19px;
                    color: #354049;
                    white-space: nowrap;
                    overflow: hidden;
                    text-overflow: ellipsis;

                    :nth-child(1),
                    :nth-child(2) {
                        margin-bottom: 3px;
                    }
                }

                &__resource-container {
                    width: 40%;
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

    @media only screen and (max-width: 1040px) {

        .price-per-month {
            display: none;
        }

        .usage-charges-item-container__detailed-info-container__content-area__resource-container,
        .resource-header,
        .usage-charges-item-container__detailed-info-container__content-area__cost-container,
        .cost-header {
            width: 15%;
        }

        .usage-charges-item-container__detailed-info-container__content-area__period-container,
        .period-header,
        .usage-charges-item-container__detailed-info-container__content-area__usage-container,
        .usage-header {
            width: 25%;
        }
    }

    @media only screen and (max-width: 768px) {

        .usage-charges-item-container__detailed-info-container__content-area__period-container,
        .period-header {
            display: none;
        }
    }

    @media only screen and (max-width: 625px) {

        .usage-charges-item-container__detailed-info-container__content-area__usage-container,
        .usage-header {
            display: none;
        }

        .usage-charges-item-container__detailed-info-container__content-area__resource-container,
        .usage-charges-item-container__detailed-info-container__content-area__cost-container {
            width: auto;
        }
    }
</style>
