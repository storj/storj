// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

// @vue/component
@Component
export default class Resizable extends Vue {
    public screenWidth: number = window.innerWidth;
    public screenHeight: number = window.innerHeight;

    public get isMobile(): boolean {
        return this.screenWidth <= 550;
    }

    public get isTablet(): boolean {
        return !this.isMobile && this.screenWidth <= 800;
    }

    public mounted(): void {
        window.addEventListener('resize', this.onResize);
    }

    public beforeDestroy(): void {
        window.removeEventListener('resize', this.onResize);
    }

    private onResize(): void {
        this.screenWidth = window.innerWidth;
        this.screenHeight = window.innerHeight;
    }
}
</script>
