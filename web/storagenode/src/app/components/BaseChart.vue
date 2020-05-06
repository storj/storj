// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<script lang="ts">
import { Component, Prop, Vue, Watch } from 'vue-property-decorator';

import VChart from '@/app/components/VChart.vue';

@Component ({
    components: {
        VChart,
    },
})
export default class BaseChart extends Vue {
    @Prop({default: 0})
    public width: number;
    @Prop({default: 0})
    public height: number;
    @Prop({default: false})
    public isDarkMode: boolean;

    public chartWidth: number = this.width;
    public chartHeight: number = this.height;
    /**
     * Used for chart re rendering.
     */
    public chartKey: number = 0;

    public $refs: {
        chartContainer: HTMLElement;
    };

    @Watch('width')
    @Watch('isDarkMode')
    public rebuildChart(): void {
        this.chartHeight = this.height;
        this.chartWidth = this.width;
        this.chartKey += 1;
    }
}
</script>

<style scoped lang="scss">
    .chart {
        width: 100%;
        height: 100%;

        &__data-dimension {
            font-size: 13px;
            color: #586c86;
            margin: 0 0 5px 31px !important;
            font-family: 'font_medium', sans-serif;
        }
    }

    @media screen and (max-width: 400px) {

        .chart-container {
            width: calc(100% - 36px);
            padding: 24px 18px;
        }
    }
</style>
