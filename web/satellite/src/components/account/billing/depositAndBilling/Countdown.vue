// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="countdown-container">
        <div v-if="isExpired">{{date}}</div>
        <div class="row" v-else>
            <p>Expires in </p>
            <p class="digit margin">{{ minutes | two_digits }}</p>
            <p>:</p>
            <p class="digit">{{ seconds | two_digits }}</p>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { BillingHistoryItemType } from '@/types/payments';

@Component
export default class Countdown extends Vue {
    @Prop({default: () => new Date()})
    private readonly expirationDate: Date;
    @Prop({default: () => new Date()})
    private readonly startDate: Date;
    @Prop({default: 0})
    private readonly type: BillingHistoryItemType;

    private readonly expirationDateTime: number;
    private now = Math.trunc((new Date()).getTime() / 1000);
    private intervalID;

    public isExpired: boolean = true;

    public constructor() {
        super();

        this.expirationDateTime = Math.trunc(new Date(this.expirationDate).getTime() / 1000);
        this.ready();
    }

    public get date(): string {
        if (this.type === BillingHistoryItemType.Transaction) {
            return this.startDate.toLocaleDateString();
        }

        return `${this.startDate.toLocaleDateString()} - ${this.expirationDate.toLocaleDateString()}`;
    }

    public get seconds(): number {
        return (this.expirationDateTime - this.now) % 60;
    }

    public get minutes(): number {
        return Math.trunc((this.expirationDateTime - this.now) / 60) % 60;
    }

    public ready() {
        this.intervalID = setInterval(() => {
            if ((this.expirationDateTime - this.now) < 0) {
                this.isExpired = true;
                clearInterval(this.intervalID);

                return;
            }

            if (this.isExpired) {
                this.isExpired = false;
            }

            this.now = Math.trunc((new Date()).getTime() / 1000);
        }, 1000);
    }
}
</script>

<style scoped lang="scss">
    .digit {
        font-family: 'font_bold', sans-serif;
    }

    .margin {
        margin-left: 5px;
    }

    .row {
        display: flex;
        align-items: center;
        justify-content: flex-start;
    }
</style>
