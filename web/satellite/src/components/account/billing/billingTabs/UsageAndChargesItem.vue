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
                >{{ item.summary() | centsToDollars }}
                </span>
            </span>
        </div>
        <div v-if="isDetailedInfoShown" class="usage-charges-item-container__detailed-info-container">
            <div class="divider" />
            <div class="usage-charges-item-container__detailed-info-container__info-header">
                <span class="resource-header">RESOURCE</span>
                <span class="period-header">PERIOD</span>
                <span class="usage-header">USAGE</span>
                <span class="cost-header">COST</span>
            </div>
            <div class="usage-charges-item-container__detailed-info-container__content-area">
                <div class="usage-charges-item-container__detailed-info-container__content-area__resource-container">
                    <p>Storage <span class="price-per-month">({{ storagePrice }} per Gigabyte-Month)</span></p>
                    <p>Egress <span class="price-per-month">({{ egressPrice }} per GB)</span></p>
                    <p>Segments <span class="price-per-month">({{ segmentPrice }} per Segment-Month)</span></p>
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
            <div class="usage-charges-item-container__detailed-info-container__footer" />
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';

import { ProjectUsageAndCharges, ProjectUsagePriceModel } from '@/types/payments';
import { Project } from '@/types/projects';
import { Size } from '@/utils/bytesSize';
import { SHORT_MONTHS_NAMES } from '@/utils/constants/date';
import { decimalShift, formatPrice, CENTS_MB_TO_DOLLARS_GB_SHIFT } from '@/utils/strings';
import { useStore } from '@/utils/hooks';

import GreyChevron from '@/../static/images/common/greyChevron.svg';

/**
 * HOURS_IN_MONTH constant shows amount of hours in 30-day month.
 */
const HOURS_IN_MONTH = 720;

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
    const project: Project | undefined = projects.find(project => project.id === props.item.projectId);

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
    return formatPrice(decimalShift(priceModel.value.storageMBMonthCents, CENTS_MB_TO_DOLLARS_GB_SHIFT));
});

/**
 * Returns egress price per GB.
 */
const egressPrice = computed((): string => {
    return formatPrice(decimalShift(priceModel.value.egressMBCents, CENTS_MB_TO_DOLLARS_GB_SHIFT));
});

/**
 * Returns segment price.
 */
const segmentPrice = computed((): string => {
    return formatPrice(decimalShift(priceModel.value.segmentMonthCents, 2));
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
        font-size: 16px;
        line-height: 21px;
        padding: 20px;
        margin-top: 10px;
        font-family: 'font_regular', sans-serif;
        background-color: #fff;
        border-radius: 8px;
        box-shadow: 0 0 20px rgb(0 0 0 / 4%);

        &__summary {
            display: flex;
            justify-content: space-between;
            align-items: center;
            flex-wrap: wrap;
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
                font-size: 16px;
                line-height: 21px;
                text-align: right;
                color: #354049;
                display: flex;
                align-items: center;
            }

            &__amount {
                font-size: 24px;
                line-height: 31px;
                font-weight: 800;
                text-align: right;
                color: #000;
            }
        }

        &__detailed-info-container {
            display: flex;
            flex-direction: column;
            align-items: flex-start;
            justify-content: space-between;

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
                padding-top: 10px;
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

            &__footer {
                display: flex;
                justify-content: space-between;
                align-content: center;
                flex-wrap: wrap;
                padding-top: 10px;
                width: 100%;

                &__payment-type {
                    display: flex;
                    flex-direction: column;
                    padding-top: 10px;

                    &__method {
                        color: var(--c-grey-6);
                        font-weight: 600;
                        font-size: 12px;
                    }

                    &__type {
                        font-weight: 400;
                        font-size: 16px;
                    }
                }

                &__buttons {
                    display: flex;
                    align-self: center;
                    flex-wrap: wrap;
                    padding-top: 10px;

                    &__assigned {
                        padding: 5px 10px;
                    }

                    &__none-assigned {
                        padding: 5px 10px;
                        margin-left: 10px;
                    }
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

    @media only screen and (max-width: 507px) {

        .usage-charges-item-container__detailed-info-container__footer__buttons__none-assigned {
            margin-left: 0;
            margin-top: 5px;
        }
    }
</style>
