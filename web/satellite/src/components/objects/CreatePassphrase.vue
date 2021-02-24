// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="create-pass">
        <GeneratePassphrase
            :is-loading="isLoading"
            :on-button-click="onNextClick"
            :set-parent-passphrase="setPassphrase"
        />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import GeneratePassphrase from '@/components/common/GeneratePassphrase.vue';

@Component({
    components: {
        GeneratePassphrase,
    },
})
export default class CreatePassphrase extends Vue {
    private isLoading: boolean = false;

    public passphrase: string = '';

    /**
     * Sets passphrase from child component.
     */
    public setPassphrase(passphrase: string): void {
        this.passphrase = passphrase;
    }

    /**
     * Holds on next button click logic.
     */
    public onNextClick(): void {
        if (this.isLoading) return;

        this.isLoading = true;
    }
}
</script>

<style scoped lang="scss">
    .create-pass {
        margin-top: 150px;
    }
</style>
