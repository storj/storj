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
            <div class="modal-dialog modal-dialog-centered">
                <div class="modal-content text-center border-0 p-2 p-sm-4">
                    <div class="modal-header border-0">
                        <h5 class="modal-title pt-2">Share</h5>
                        <button
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
                            <hr>
                            <div class="social-share-icons my-4">
                                <p>Share this link via</p>
                                <ShareContainer :link="objectLink" />
                            </div>

                            <hr>

                            <p class="my-4">Or copy link</p>

                            <div class="input-group my-4">
                                <input
                                    id="url"
                                    class="form-control"
                                    type="url"
                                    :value="objectLink"
                                    aria-describedby="btn-copy-link"
                                    readonly
                                >
                                <div class="input-group-append">
                                    <button
                                        id="btn-copy-link"
                                        type="button"
                                        name="copy"
                                        class="
											btn
											btn-outline-secondary
											btn-copy-link
										"
                                        @click="copy"
                                    >
                                        {{ copyText }}
                                    </button>
                                </div>
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

// @vue/component
@Component({
    components: {
        ShareContainer,
    },
})
export default class FileShareModal extends Vue {
    public objectLink = '';
    public copyText = 'Copy Link';

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
    }

    /**
     * Copy the selected link to the user's clipboard and update the copyText accordingly.
     */
    public async copy(): Promise<void> {
        await this.$copyText(this.objectLink);
        this.copyText = 'Copied!';
        setTimeout(() => {
            this.copyText = 'Copy Link';
        }, 2000);
    }

    /**
     * Close the FileShareModal.
     */
    public close(): void {
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

<style scoped>
.modal-open {
    display: block !important;
}

.modal-title {
    word-break: break-word;
    font-weight: bold;
}

.closex {
    cursor: pointer;
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
</style>
