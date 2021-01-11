// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="add-new-node">
        <v-button :with-plus="true" label="New Node" :on-press="openModal" width="152px" />
        <v-modal v-if="isAddNewNodeModalShown" @close="closeModal">
            <h2 slot="header">Add New Node</h2>
            <div class="add-new-node__body" slot="body">
                <headered-input
                    class="add-new-node__body__input"
                    label="Public IP Address"
                />
                <headered-input
                    class="add-new-node__body__input"
                    label="API Key"
                />
                <headered-input
                    class="add-new-node__body__input"
                    label="Displayed Name"
                />
            </div>
            <div class="add-new-node__footer" slot="footer">
                <v-button label="Cancel" :is-white="true" width="205px" :on-press="closeModal" />
                <v-button label="Continue" width="205px" />
            </div>
        </v-modal>
    </div>

</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import HeaderedInput from '@/app/components/common/HeaderedInput.vue';
import VButton from '@/app/components/common/VButton.vue';
import VModal from '@/app/components/common/VModal.vue';

@Component({
    components: {
        VButton,
        HeaderedInput,
        VModal,
    },
})
export default class AddNewNode extends Vue {
    public isAddNewNodeModalShown: boolean = false;

    public openModal(): void {
        this.isAddNewNodeModalShown = true;
    }

    public closeModal(): void {
        this.isAddNewNodeModalShown = false;
    }
}
</script>

<style lang="scss">
    .add-new-node {

        h2 {
            margin: 0;
            font-size: 32px;
        }

        &__body {
            width: 441px;

            &__input:not(:first-of-type) {
                margin-top: 20px;
            }
        }

        &__footer {
            width: 441px;
            display: flex;
            align-items: center;
            justify-content: space-between;
        }
    }
</style>
