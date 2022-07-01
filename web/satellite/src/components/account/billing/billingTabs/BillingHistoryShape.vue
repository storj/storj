// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="billing-history">
        <div class="billing-history__common-info">
            <span class="name-container" :title="historyItem.start">
                <Calendar />
                <p class="name_date">{{ historyItem.start }}</p>
            </span>
        </div>
        <div class="billing-history__common-info">
            <span class="name-container" :title="historyItem.status">
                <CheckIcon class="checkmark" />
                <p class="name_status">{{ historyItem.status }}</p>
            </span>
        </div>
        <div class="billing-history__common-info">
            <div class="name-container" :title="historyItem.amount">
                <p class="name_amount">{{ historyItem.amount }}</p>
            </div>
        </div>
        <div v-if="historyItem.link" class="billing-history__common-info">
            <a href="historyItem.link" download>
                <div class="name-container" :title="historyItem.link">
                    <p class="name_download">Invoice PDF</p>
                </div>
            </a>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { PaymentsHistoryItem } from '@/types/payments';

import { AccessGrant } from '@/types/accessGrants';
import CheckIcon from '@/../static/images/billing/check-green-circle.svg';
import Calendar from '@/../static/images/billing/calendar.svg';

// @vue/component
@Component({
    components: {
        Calendar,
        CheckIcon,
    },
})
export default class BillingHistoryShape extends Vue {
    @Prop({ default: new PaymentsHistoryItem('', '', 0, 0, '', '', new Date(), new Date(), 0, 0) })
    @Prop({ default: new AccessGrant('', '', new Date(), '') })
    private readonly historyItem: PaymentsHistoryItem;
    private popupVisible = false;

    public togglePopupVisibility(): void {
        this.popupVisible = !this.popupVisible;
    }
}
</script>

<style scoped lang="scss">
    @mixin popup-menu-button {
        padding: 0 15px;
        height: 50%;
        line-height: 55px;
        text-align: left;
        font-family: 'font_regular', sans-serif;
        color: #1b2533;
        transition: 100ms;
    }

    .checkmark {
        margin-top: 3px;
        margin-left: 37px;
    }

    .billing-history {
        display: flex;
        align-items: center;
        justify-content: flex-start;
        height: 64px;
        background-color: #fff;
        border: 1px solid #e5e7eb;
        border-bottom: 0;
        width: 78%;

        &__common-info {
            margin-left: 10px;
            display: flex;
            align-items: center;
            justify-content: flex-start;
            width: 60%;
        }
    }

    .checkbox-container {
        margin-left: 28px;
        min-width: 21px;
        min-height: 21px;
        border-radius: 4px;
        border: 1px solid #1b2533;

        &__image {
            display: none;
        }
    }

    .name-container {
        // max-width: calc(100% - 131px);
        display: flex;
        margin-right: 15px;
    }

    .name_date {
        margin-top: 6px;
        font-family: 'font_bold', sans-serif;
        font-size: 16px;
        line-height: 21px;
        color: #354049;
        margin-left: 15px;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }

    .name_status {
        font-family: 'font_bold', sans-serif;
        font-style: normal;
        font-weight: 400;
        font-size: 14px;
        line-height: 20px;
        color: #111827;
        margin-left: 6px;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }

    .name_amount {
        font-family: 'font_bold', sans-serif;
        font-style: normal;
        font-weight: 400;
        font-size: 14px;
        line-height: 20px;
        color: #111827;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
        margin-left: -8px;
    }

    .name_downloaod {
        font-family: 'font_bold', sans-serif;
        font-size: 16px;
        line-height: 21px;
        color: #354049;
        margin-left: 5px;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }

    .date {
        font-family: 'font_regular', sans-serif;
        font-size: 16px;
        line-height: 21px;
        color: #354049;
        margin: 0;
    }

    .ellipses {
        margin: 0 auto 20px;
        font-size: 30px;
        font-weight: 1000;
        color: #7c8794;
        cursor: pointer;
    }

    .popup-menu {
        width: 160px;
        height: 100px;
        position: absolute;
        right: 70px;
        bottom: -90px;
        z-index: 1;
        background: #fff;
        border-radius: 10px;
        box-shadow: 0 20px 34px rgb(10 27 44 / 28%);

        &__popup-details {
            @include popup-menu-button;

            border-radius: 10px 10px 0 0;

            &:hover {
                background-color: #354049;
                cursor: pointer;
                color: #fff;
            }
        }

        &__popup-divider {
            height: 1px;
            background-color: #e5e7eb;
        }

        &__popup-delete {
            @include popup-menu-button;

            border-radius: 0 0 10px 10px;

            &:hover {
                background-color: #b53737;
                cursor: pointer;
                color: #fff;
            }
        }
    }

    .date-item-container {
        width: 50%;
    }

    .menu-item-container {
        width: 10%;
        position: relative;
    }
</style>
