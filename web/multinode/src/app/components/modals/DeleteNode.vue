// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="delete-node">
        <div @click="openModal" class="delete-node__button">Delete Node</div>
        <v-modal v-if="isModalShown" @close="closeModal">
            <h2 slot="header">Delete this node?</h2>
            <div class="delete-node__body" slot="body">
                <div class="delete-node__body__node-id-container">
                    <span>{{ nodeId }}</span>
                </div>
            </div>
            <div class="delete-node__footer" slot="footer">
                <v-button label="Cancel" :is-white="true" width="205px" :on-press="closeModal" />
                <v-button label="Delete" width="205px" :on-press="onDelete"/>
            </div>
        </v-modal>
    </div>

</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import VButton from '@/app/components/common/VButton.vue';
import VModal from '@/app/components/common/VModal.vue';

@Component({
    components: {
        VButton,
        VModal,
    },
})
export default class AddNewNode extends Vue {
    @Prop({default: ''})
    public nodeId: string;

    public isModalShown: boolean = false;

    private isLoading: boolean = false;

    public openModal(): void {
        this.isModalShown = true;
    }

    public closeModal(): void {
        this.isLoading = false;
        this.isModalShown = false;
    }

    public async onDelete(): Promise<void> {
        if (this.isLoading) return;

        this.isLoading = true;

        try {
            await this.$store.dispatch('nodes/delete', this.nodeId);
            this.closeModal();
        } catch (error) {
            console.error(error.message);
            this.isLoading = false;
        }
    }
}
</script>

<style lang="scss">
    .delete-node {

        h2 {
            margin: 0;
            font-size: 32px;
        }

        &__button {
            width: 100%;
            box-sizing: border-box;
            padding: 16px;
            cursor: pointer;
            text-align: left;
            font-family: 'font_medium', sans-serif;
            font-size: 14px;
            color: var(--c-title);

            &:hover {
                background: var(--c-background);
            }
        }

        &__body {
            width: 460px;

            &__node-id-container {
                width: 100%;
                box-sizing: border-box;
                padding: 10px 12px;
                font-family: 'font_regular', sans-serif;
                font-size: 14px;
                color: var(--c-title);
                background: var(--c-background);
                border-radius: 32px;
            }
        }

        &__footer {
            width: 460px;
            display: flex;
            align-items: center;
            justify-content: space-between;
        }

        &__item {
            box-sizing: border-box;
            padding: 16px;
            cursor: pointer;
            text-align: left;

            &:hover {
                background: var(--c-background);
            }
        }
    }
</style>
