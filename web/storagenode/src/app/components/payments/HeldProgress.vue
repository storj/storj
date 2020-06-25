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
                <div class="divider"></div>
                <h4 class="label">{{ step.label }}</h4>
            </div>
        </div>
        <div class="held-progress__border"></div>
        <p class="held-progress__main-text">It is your <span class="bold">{{ monthsOnNetwork }} month</span> on network</p>
    </div>
</template>

<script lang="ts">
import { Component, Vue, Watch } from 'vue-property-decorator';

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

@Component
export default class HeldProgress extends Vue {
    public steps: HeldStep[] = [];

    /**
     * Returns approximated number of months that node is online.
     */
    public get monthsOnNetwork(): number {
        return this.$store.getters.monthsOnNetwork;
    }

    /**
     * Lifecycle hook after initial render.
     * Builds held steps.
     */
    public mounted(): void {
        this.buildSteps();
    }

    /**
     * Builds held steps depends on node`s months online.
     */
    @Watch('monthsOnNetwork')
    private buildSteps(): void {
        this.steps = [
            new HeldStep(
                '75%',
                'Month 1-3',
                this.monthsOnNetwork > 0 && this.monthsOnNetwork <= 3,
                false,
            ),
            new HeldStep(
                '50%',
                'Month 4-6',
                this.monthsOnNetwork > 3 && this.monthsOnNetwork <= 6,
                this.monthsOnNetwork < 4,
            ),
            new HeldStep(
                '25%',
                'Month 7-9',
                this.monthsOnNetwork > 6 && this.monthsOnNetwork <= 9,
                this.monthsOnNetwork < 7,
            ),
            new HeldStep(
                '0%',
                'Month 10-15',
                this.monthsOnNetwork > 9 && this.monthsOnNetwork <= 15,
                this.monthsOnNetwork < 10,
            ),
            new HeldStep(
                '+50%',
                'Month 15',
                this.monthsOnNetwork > 15,
                this.monthsOnNetwork < 15,
            ),
        ];
    }
}
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
            margin: 16px 0 8px 0;

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
            grid-column-gap: 2px;

            &__step {
                @include step();

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
            margin: 24px 0 20px 0;
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

    @media screen and (max-width: 640px) {

        .label {
            text-align: center;
        }
    }
</style>
