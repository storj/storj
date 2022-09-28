// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div @click="stopClickPropagation">
        <div
            id="share-modal"
            class="modal fade show modal-open"
            tabindex="-1"
            aria-labelledby="shareModalLabel"
            aria-hidden="true"
        >
            <div id="modal-content-container" class="modal-dialog modal-dialog-centered">
                <div id="modal-content" class="modal-content text-center border-0 p-2 p-sm-4">
                    <div id="header" class="modal-header border-0">
                        <h5 class="modal-title pt-2">Share File</h5>
                        <button
                            id="close-btn"
                            type="button"
                            class="close"
                            data-dismiss="modal"
                            aria-label="Close"
                            @click="close"
                        >
                            <span
                                aria-hidden="true"
                                aria-roledescription="close-share-modal"
                            >&times;</span>
                        </button>
                    </div>
                    <div class="modal-body pt-0 text-left">
                        <div>
                            <div class="social-share-icons my-4">
                                <ShareContainer :link="objectLink" />
                            </div>

                            <p id="copy-text" class="my-4 text-center">Or copy link:</p>
                            
                            <VLoader v-if="isLoading" width="20px" height="20px" />

                            <div v-if="!isLoading" class="input-group">
                                <input
                                    id="url"
                                    class="form-control"
                                    type="url"
                                    :value="objectLink"
                                    aria-describedby="btn-copy-link"
                                    readonly
                                >
                                <VButton
                                    :label="copyButtonState === ButtonStates.Copy ? 'Copy' : 'Copied'"
                                    width="114px"
                                    height="40px"
                                    :on-press="copy"
                                    :is-disabled="isLoading"
                                    :is-green-white="copyButtonState === ButtonStates.Copied"
                                    :icon="copyButtonState === ButtonStates.Copied ? 'none' : 'copy'"
                                />
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
        <div id="backdrop" class="modal-backdrop fade show modal-open" />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import ShareContainer from '@/components/common/share/ShareContainer.vue';
import VLoader from '@/components/common/VLoader.vue';
import VButton from '@/components/common/VButton.vue';

enum ButtonStates {
    Copy,
    Copied,
}

// @vue/component
@Component({
    components: {
        ShareContainer,
        VLoader,
        VButton,
    },
})
export default class FileShareModal extends Vue {
    private readonly ButtonStates = ButtonStates;

    public objectLink = '';
    public copyText = 'Copy';
    public isLoading = true;
    public copyButtonState = ButtonStates.Copy;

    /**
     * Retrieve the path to the current file that has the fileShareModal opened from the store.
     */
    private get filePath(): void {
        return this.$store.state.files.fileShareModal;
    }

    /**
     * Set the objectLink by calling the store's fetchSharedLink function.
     */
    public async created(): Promise<void> {
        this.objectLink = await this.$store.state.files.fetchSharedLink(
            this.filePath,
        );
        this.isLoading = false;
    }

    /**
     * Copy the selected link to the user's clipboard and update the copyText accordingly.
     */
    public async copy(): Promise<void> {
        await this.$copyText(this.objectLink);
        this.copyButtonState = ButtonStates.Copied;

        setTimeout(() => {
            this.copyButtonState = ButtonStates.Copy;
        }, 2000);
    }

    /**
     * Close the FileShareModal.
     */
    public close(): void {
        if (this.isLoading) return;

        this.$store.commit('files/closeFileShareModal');
    }

    /**
     * Stop the propagation of a click event only if the target is an element without share-modal as the id.
     */
    public stopClickPropagation(e: Event): void {
        const target = e.target as HTMLElement;
        if (target.id !== 'share-modal') {
            e.stopPropagation();
        }
    }
}
</script>

<style scoped lang="scss">
.modal-open {
    display: block !important;
}

.modal-title {
    word-break: break-word;
    font-weight: bold;
    font-family: 'font_bold', sans-serif;
    font-size: 22px;
    line-height: 29px;
}

.file-browser .modal-dialog-centered {
    display: block;
}

#header {
    align-items: center;
    justify-content: center;
}

#close-btn {
    position: absolute;
    right: 24px;
    top: 24px;
    font-size: 30px;
}

#share-modal {
    z-index: 1070;
}

#backdrop {
    z-index: 1060 !important;
}

.btn-copy-link {
    border-top-right-radius: 4px;
    border-bottom-right-radius: 4px;
    font-size: 14px;
    padding: 0 16px;
}

#modal-content {
    padding: 35px !important;
    border-radius: 10px;

    @media screen and (max-width: 430px) {
        padding: 20px;
    }
}

#modal-content-container {
    max-width: 585px;
}

#url {
    background: none;
    border: none;
    font-size: 14px;
    color: #56606d;
    max-width: 340px;
    width: 100%;
    flex-wrap: inherit !important;
}

#copy-text {
    font-family: 'font_medium', sans-serif;
    font-size: 14px;
    color: #354049;
    margin-bottom: 15px !important;
}

.input-group {
    border: 1px solid #c8d3de;
    background: #fafafb;
    padding: 10px;
    display: flex;
    justify-content: space-between;
    border-radius: 8px;
    width: 100%;
    height: 67px;
    flex-wrap: inherit !important;
    margin-bottom: 0 !important;
    align-items: center !important;
}

</style>
