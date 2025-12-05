// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="held-progress">
        <div class="held-progress__steps-area">
            <div
                v-for="step in steps"
                :key="step.amount"
                :class="step.className"
            >
                <h1 class="amount">{{ step.amount }}</h1>
                <div class="divider" />
                <h4 class="label">{{ step.label }}</h4>
            </div>
        </div>
        <div class="held-progress__border" />
        <p class="held-progress__main-text">It is your <span class="bold">{{ monthsOnNetwork }} month</span> on network</p>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';

import { useNodeStore } from '@/app/store/modules/nodeStore';
import { getMonthsBeforeNow } from '@/app/utils/payout';

const nodeStore = useNodeStore();

class HeldStep {
    public constructor(
        public amount: string = '0%',
        public label: string = '',
        public active: boolean = false,
        public inFuture: boolean = true,
    ) {}

    public get className(): string {
        if (this.active) return 'held-progress__steps-area__step--active';
        if (this.inFuture) return 'held-progress__steps-area__step--future';

        return 'held-progress__steps-area__step';
    }
}

const MONTHS_BREAKPOINTS: number[] = [0, 3, 6, 9, 15];

const steps = ref<HeldStep[]>([]);

const monthsOnNetwork = computed<number>(() => {
    return getMonthsBeforeNow(nodeStore.state.selectedSatellite.joinDate);
});

function buildSteps(): void {
    steps.value = [
        new HeldStep(
            '75%',
            `Month ${MONTHS_BREAKPOINTS[0] + 1}-${MONTHS_BREAKPOINTS[1]}`,
            monthsOnNetwork.value > MONTHS_BREAKPOINTS[0] && monthsOnNetwork.value <= MONTHS_BREAKPOINTS[1],
            false,
        ),
        new HeldStep(
            '50%',
            `Month ${MONTHS_BREAKPOINTS[1] + 1}-${MONTHS_BREAKPOINTS[2]}`,
            monthsOnNetwork.value > MONTHS_BREAKPOINTS[1] && monthsOnNetwork.value <= MONTHS_BREAKPOINTS[2],
            monthsOnNetwork.value < MONTHS_BREAKPOINTS[1] + 1,
        ),
        new HeldStep(
            '25%',
            `Month ${MONTHS_BREAKPOINTS[2] + 1}-${MONTHS_BREAKPOINTS[3]}`,
            monthsOnNetwork.value > MONTHS_BREAKPOINTS[2] && monthsOnNetwork.value <= MONTHS_BREAKPOINTS[3],
            monthsOnNetwork.value < MONTHS_BREAKPOINTS[2] + 1,
        ),
        new HeldStep(
            '0%',
            `Month ${MONTHS_BREAKPOINTS[3] + 1}-${MONTHS_BREAKPOINTS[4]}`,
            monthsOnNetwork.value > MONTHS_BREAKPOINTS[3] && monthsOnNetwork.value <= MONTHS_BREAKPOINTS[4],
            monthsOnNetwork.value < MONTHS_BREAKPOINTS[3] + 1,
        ),
        new HeldStep(
            '+50%',
            `Month ${MONTHS_BREAKPOINTS[4] + 1}`,
            monthsOnNetwork.value > MONTHS_BREAKPOINTS[4],
            monthsOnNetwork.value < MONTHS_BREAKPOINTS[4] + 1,
        ),
    ];
}

watch(monthsOnNetwork, buildSteps);

onMounted(() => {
    buildSteps();
});
</script>

<style scoped lang="scss">
    p,
    h1,
    h4 {
        margin: 0;
    }

    @mixin step($active: false, $future: false) {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;

        .amount {
            font-family: 'font_bold', sans-serif;
            font-size: 20px;
            color: #224ca5;
            opacity: 0.4;

            @if $active {
                opacity: 1;
            }

            @if $future {
                color: #909bad;
            }
        }

        .divider {
            width: 100%;
            height: 4px;
            background: #224ca5;
            mix-blend-mode: normal;
            opacity: 0.4;
            border-radius: 4px;
            margin: 16px 0 8px;

            @if $active {
                opacity: 1;
            }

            @if $future {
                background: #909bad;
            }
        }

        .label {
            font-size: 14px;
            color: #586c86;
        }
    }

    .held-progress {
        display: flex;
        flex-direction: column;
        width: 100%;
        background: var(--block-background-color);
        border: 1px solid var(--block-border-color);
        box-sizing: border-box;
        border-radius: 12px;
        padding: 29px;
        font-family: 'font_regular', sans-serif;

        &__steps-area {
            display: grid;
            grid-template-columns: repeat(5, 1fr);
            column-gap: 2px;

            &__step {
                @include step;

                &--active {
                    @include step(true);
                }

                &--future {
                    @include step($future: true);
                }
            }
        }

        &__border {
            width: 100%;
            height: 1px;
            background: #a9b5c1;
            opacity: 0.3;
            margin: 24px 0 20px;
        }

        &__main-text {
            font-size: 16px;
            color: var(--regular-text-color);

            .bold {
                font-family: 'font_bold', sans-serif;
            }
        }

        &__hint {
            font-size: 13px;
            line-height: 15px;
            color: #9b9db1;
        }
    }

    @media screen and (width <= 640px) {

        .label {
            text-align: center;
        }
    }
</style>
