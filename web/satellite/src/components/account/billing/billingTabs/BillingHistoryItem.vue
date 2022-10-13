// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <tr @click="downloadInvoice">
        <th class="align-left data mobile">
            <div class="few-items">
                <p class="array-val date">
                    <span><Calendar /></span>
                    <span>{{ item.formattedStart }}</span>
                </p>
                <p class="array-val status">
                    <span v-if="item.status === 'paid'"> <CheckIcon class="checkmark" /> </span>
                    <span>{{ item.formattedStatus }}</span>
                </p>
                <p class="array-val">
                    {{ item.amount | centsToDollars }}
                </p>
            </div>
        </th>
        <fragment>
            <th class="align-left data tablet-laptop">
                <p class="date">
                    <span><Calendar /></span>
                    <span>{{ item.formattedStart }}</span>
                </p>
            </th>
            <th class="align-left data tablet-laptop">
                <p class="status">
                    <span v-if="item.status === 'paid'"> <CheckIcon class="checkmark" /> </span>
                    <span>{{ item.formattedStatus }}</span>
                </p>
            </th>
            <th class="align-left data tablet-laptop">
                <p>
                    {{ item.amount | centsToDollars }}
                </p>
            </th>
            <th class="align-left data tablet-laptop">
                <a :href="item.link" download>Invoice PDF</a>
            </th>
        </fragment>
    </tr>
</template>

<script lang="ts">
import { Component, Prop } from 'vue-property-decorator';
import { Fragment } from 'vue-fragment';

import { PaymentsHistoryItem } from '@/types/payments';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';

import Resizable from '@/components/common/Resizable.vue';

import CheckIcon from '@/../static/images/billing/check-green-circle.svg';
import Calendar from '@/../static/images/billing/calendar.svg';

// @vue/component
@Component({
    components: {
        Calendar,
        CheckIcon,
        Fragment,
    },
})
export default class BillingHistoryItem extends Resizable {
    @Prop({ default: new PaymentsHistoryItem('', '', 0, 0, '', '', new Date(), new Date(), 0, 0) })
    private readonly item: PaymentsHistoryItem;

    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    public downloadInvoice() {
        this.analytics.eventTriggered(AnalyticsEvent.INVOICE_DOWNLOADED);

        if (this.isMobile || this.isTablet)
            window.open(this.item.link, '_blank', 'noreferrer');
    }
}
</script>

<style scoped lang="scss">
    a {
        color: #0149ff;
        text-decoration: underline;
    }

    .date {
        display: flex;
        gap: 0.7rem;
        align-items: center;
    }

    .status {
        display: flex;
        gap: 0.7rem;
        align-items: center;
    }

    .few-items {
        display: flex;
        flex-direction: column;
        justify-content: space-between;
    }

    .array-val {
        font-family: 'font_regular', sans-serif;
        font-size: 0.75rem;
        line-height: 1.25rem;

        &:first-of-type {
            font-family: 'font_bold', sans-serif;
            font-size: 0.875rem;
            margin-bottom: 3px;
        }
    }

    @media only screen and (max-width: 425px) {

        .mobile {
            display: table-cell;
        }

        .tablet-laptop {
            display: none;
        }
    }

    @media only screen and (min-width: 426px) {

        .tablet-laptop {
            display: table-cell;
        }

        .mobile {
            display: none;
        }
    }
</style>
