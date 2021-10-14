<script lang="ts"> /* eslint-disable */ </script>

// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<style scoped>
/* stylelint-disable */

.modal-header {
	border-bottom-color: #eeeeee;
	background-color: #fafafa;
}

.file-preview-wrapper {
	/* Testing background for file preview */
	/* background: #000; */
	background: #f9fafc;
	height: 100%;
	min-height: 75vh;
	border-right: 1px solid #eee;
}

.btn-demo {
	margin: 15px;
	padding: 10px 15px;
	border-radius: 0;
	font-size: 16px;
	background-color: #ffffff;
}

.btn-demo:focus {
	outline: 0;
}

.closex {
	cursor: pointer;
}

.modal-open {
	display: block !important;
}

.file-path {
	display: inline-block;
	font-weight: bold;
	max-width: 100%;
	position: relative;
	font-size: 18px;
}

.preview {
	width: 100%;
}
.preview-placeholder {
	background: #f9fafc;
	width: 100%;
	height: 100%;
}

.object-map {
	width: 100%;
}

.storage-nodes {
	padding: 5px;
	background: rgba(0, 0, 0, 0.8);
	font-weight: normal;
	color: white;
	font-size: 0.8rem;
}

.size {
	font-size: 0.9rem;
	font-weight: normal;
}

.btn {
	line-height: 2.4;
}
.btn-primary {
	background: #376fff;
	border-color: #376fff;
}
.btn-light {
	background: #e6e9ef;
	border-color: #e6e9ef;
}
.share-btn {
	font-weight: bold;
}
.text-lighter {
	color: #768394;
}

.btn-copy-link {
	border-top-right-radius: 4px;
	border-bottom-right-radius: 4px;
	font-size: 14px;
	padding: 0 16px;
}
</style>

<template>
	<div class="container demo" v-on:click="stopClickPropagation">
		<div
			class="modal right fade in show modal-open"
			id="detail-modal"
			tabindex="-1"
			role="dialog"
			aria-labelledby="modalLabel"
		>
			<div
				class="modal-dialog modal-xl modal-dialog-centered"
				role="document"
			>
				<div class="modal-content">
					<div class="modal-body p-0">
						<div class="container-fluid p-0">
							<div class="row">
								<div class="col-6 col-lg-8">
									<div
										class="
											file-preview-wrapper
											d-flex
											align-items-center
											justify-content-center
										"
									>
										<img
											class="preview img-fluid"
											v-if="previewIsImage"
											v-bind:src="preSignedUrl"
											aria-roledescription="image-preview"
										/>

										<video
											class="preview"
											controls
											v-if="previewIsVideo"
											v-bind:src="preSignedUrl"
											aria-roledescription="video-preview"
										></video>

										<audio
											class="preview"
											controls
											v-if="previewIsAudio"
											v-bind:src="preSignedUrl"
											aria-roledescription="audio-preview"
										></audio>

										<svg
											v-if="placeHolderDisplayable"
											width="300"
											height="172"
											viewBox="0 0 300 172"
											fill="none"
											xmlns="http://www.w3.org/2000/svg"
											class="
												preview-placeholder
												img-fluid
											"
											aria-roledescription="preview-placeholder"
										>
											<path
												d="M188.5 140C218.047 140 242 116.047 242 86.5C242 56.9528 218.047 33 188.5 33C158.953 33 135 56.9528 135 86.5C135 116.047 158.953 140 188.5 140Z"
												fill="white"
											/>
											<path
												d="M123.5 167C147.524 167 167 147.524 167 123.5C167 99.4756 147.524 80 123.5 80C99.4756 80 80 99.4756 80 123.5C80 147.524 99.4756 167 123.5 167Z"
												fill="white"
											/>
											<path
												d="M71.5 49C78.9558 49 85 42.9558 85 35.5C85 28.0442 78.9558 22 71.5 22C64.0442 22 58 28.0442 58 35.5C58 42.9558 64.0442 49 71.5 49Z"
												fill="white"
											/>
											<path
												d="M262.5 143C268.851 143 274 137.851 274 131.5C274 125.149 268.851 120 262.5 120C256.149 120 251 125.149 251 131.5C251 137.851 256.149 143 262.5 143Z"
												fill="white"
											/>
											<path
												d="M185.638 64.338L191 57M153 109L179.458 72.7948L153 109Z"
												stroke="#276CFF"
												stroke-width="2"
												stroke-linecap="round"
											/>
											<path
												d="M121.08 153.429L115 161M153 108L127.16 144.343L153 108Z"
												stroke="#276CFF"
												stroke-width="2"
												stroke-linecap="round"
											/>
											<path
												d="M134 71L115 31M152 109L139 81L152 109Z"
												stroke="#FF458B"
												stroke-width="2"
												stroke-linecap="round"
											/>
											<path
												d="M180.73 129.5L210 151M153 108L173.027 123.357L153 108Z"
												stroke="#FF458B"
												stroke-width="2"
												stroke-linecap="round"
											/>
											<path
												d="M86.7375 77.1845L72 70M152 109L109.06 88.0667L152 109Z"
												stroke="#FFC600"
												stroke-width="2"
												stroke-linecap="round"
											/>
											<path
												d="M152.762 109.227L244.238 76.7727"
												stroke="#00E567"
												stroke-width="2"
												stroke-linecap="round"
											/>
											<path
												d="M154.5 104.5L111 131"
												stroke="#00E567"
												stroke-width="2"
												stroke-linecap="round"
											/>
											<path
												fill-rule="evenodd"
												clip-rule="evenodd"
												d="M224 57H238V71H224V57Z"
												fill="#00E567"
											/>
											<path
												fill-rule="evenodd"
												clip-rule="evenodd"
												d="M127 2H137V12H127V2Z"
												fill="#FF458B"
											/>
											<path
												fill-rule="evenodd"
												clip-rule="evenodd"
												d="M150 166H156V172H150V166Z"
												fill="#FF458B"
											/>
											<path
												fill-rule="evenodd"
												clip-rule="evenodd"
												d="M44 0H50V6H44V0Z"
												fill="#00E567"
											/>
											<path
												fill-rule="evenodd"
												clip-rule="evenodd"
												d="M294 111H300V117H294V111Z"
												fill="#276CFF"
											/>
											<path
												fill-rule="evenodd"
												clip-rule="evenodd"
												d="M0 121H6V127H0V121Z"
												fill="#276CFF"
											/>
											<path
												fill-rule="evenodd"
												clip-rule="evenodd"
												d="M268 86H274V92H268V86Z"
												fill="#FFC600"
											/>
											<path
												fill-rule="evenodd"
												clip-rule="evenodd"
												d="M28 91H46V109H28V91Z"
												fill="#FFC600"
											/>
											<path
												fill-rule="evenodd"
												clip-rule="evenodd"
												d="M181 21H203V43H181V21Z"
												fill="#276CFF"
											/>
											<path
												fill-rule="evenodd"
												clip-rule="evenodd"
												d="M154.958 55L179 79.0416V136H122V55H154.958Z"
												fill="#0218A7"
											/>
											<path
												d="M146.5 80H136.5C135.119 80 134 81.1193 134 82.5C134 83.8807 135.119 85 136.5 85H146.5C147.881 85 149 83.8807 149 82.5C149 81.1193 147.881 80 146.5 80Z"
												fill="white"
											/>
											<path
												d="M164.5 92H136.5C135.119 92 134 93.1193 134 94.5C134 95.8807 135.119 97 136.5 97H164.5C165.881 97 167 95.8807 167 94.5C167 93.1193 165.881 92 164.5 92Z"
												fill="white"
											/>
											<path
												d="M164.5 104H136.5C135.119 104 134 105.119 134 106.5C134 107.881 135.119 109 136.5 109H164.5C165.881 109 167 107.881 167 106.5C167 105.119 165.881 104 164.5 104Z"
												fill="white"
											/>
											<path
												d="M164.5 116H136.5C135.119 116 134 117.119 134 118.5C134 119.881 135.119 121 136.5 121H164.5C165.881 121 167 119.881 167 118.5C167 117.119 165.881 116 164.5 116Z"
												fill="white"
											/>
											<path
												fill-rule="evenodd"
												clip-rule="evenodd"
												d="M154.958 79.0416V55L179 79.0416H154.958Z"
												fill="#276CFF"
											/>
										</svg>
									</div>
								</div>
								<div class="col-6 col-lg-4 pr-5">
									<div class="text-right">
										<svg
											v-on:click="closeModal"
											xmlns="http://www.w3.org/2000/svg"
											width="2em"
											height="2em"
											fill="#6e6e6e"
											class="bi bi-x mt-4 closex"
											viewBox="0 0 16 16"
											id="close-modal"
										>
											<path
												d="M4.646 4.646a.5.5 0 0 1 .708 0L8 7.293l2.646-2.647a.5.5 0 0 1 .708.708L8.707 8l2.647 2.646a.5.5 0 0 1-.708.708L8 8.707l-2.646 2.647a.5.5 0 0 1-.708-.708L7.293 8 4.646 5.354a.5.5 0 0 1 0-.708z"
											/>
										</svg>
									</div>

									<div class="mb-3">
										<span class="file-path">{{
											filePath
										}}</span>
									</div>

									<p class="size mb-3">
										<span class="text-lighter mr-2"
											>Size:</span
										>
										{{ size }}
									</p>
									<p class="size mb-3">
										<span class="text-lighter mr-2"
											>Created:</span
										>
										{{ uploadDate }}
									</p>

									<button
										class="
											btn btn-primary btn-block
											mb-3
											mt-4
										"
										download
										v-on:click="download"
									>
										Download
										<svg
											width="14"
											height="15"
											viewBox="0 0 14 15"
											alt="Download"
											class="ml-2"
											fill="none"
											xmlns="http://www.w3.org/2000/svg"
										>
											<path
												d="M6.0498 7.98517V0H8.0498V7.91442L10.4965 5.46774L11.9107 6.88196L7.01443 11.7782L2.11816 6.88196L3.53238 5.46774L6.0498 7.98517Z"
												fill="white"
											/>
											<path
												d="M0 13L14 13V15L0 15V13Z"
												fill="white"
											/>
										</svg>
									</button>

									<div
										v-if="objectLink"
										class="input-group mt-4"
									>
										<input
											class="form-control"
											type="url"
											id="url"
											v-bind:value="objectLink"
											aria-describedby="generateShareLink"
											readonly
										/>
										<div class="input-group-append">
											<button
												v-on:click="copy"
												type="button"
												name="copy"
												class="
													btn
													btn-outline-secondary
													btn-copy-link
												"
												id="generateShareLink"
											>
												{{ copyText }}
											</button>
										</div>
									</div>

									<button
										v-else
										class="btn btn-light btn-block"
										v-on:click="getSharedLink"
									>
										<span class="share-btn">
											Share
											<svg
												width="16"
												height="16"
												viewBox="0 0 16 16"
												alt="Share"
												class="ml-2"
												fill="none"
												xmlns="http://www.w3.org/2000/svg"
											>
												<path
													d="M8.86084 11.7782L8.86084 3.79305L11.3783 6.31048L12.7925 4.89626L7.89622 0L2.99995 4.89626L4.41417 6.31048L6.86084 3.8638L6.86084 11.7782L8.86084 11.7782Z"
													fill="#384B65"
												/>
												<path
													d="M4.5 8.12502H0.125V15.875H15.875V8.12502H11.5V9.87502H14.125V14.125H1.875V9.87502H4.5V8.12502Z"
													fill="#384B65"
												/>
											</svg>
										</span>
									</button>

									<div
										v-if="objectMapIsLoading"
										class="
											d-flex
											justify-content-center
											text-primary
											mt-4
										"
									>
										<div
											class="spinner-border mt-3"
											role="status"
										></div>
									</div>

									<div
										class="mt-5"
										v-if="objectMapUrl !== null"
									>
										<div class="storage-nodes">
											Nodes storing this file
										</div>
										<img
											class="object-map"
											v-bind:src="objectMapUrl"
										/>
									</div>
								</div>
							</div>
						</div>
					</div>
				</div>
			</div>
		</div>
		<div class="modal-backdrop fade show modal-open" id="backdrop2"></div>
	</div>
</template>

<script>
import FileShareModal from "./FileShareModal";
import prettyBytes from "pretty-bytes";

export default {
	name: "FileModal",
	data: () => ({
		objectMapIsLoading: false,
		objectMapUrl: null,
		objectLink: null,
		copyText: "Copy Link"
	}),
	computed: {
		file() {
			return this.$store.state.files.files.find(
				(file) => file.Key === this.filePath.split("/").slice(-1)[0]
			);
		},
		filePath() {
			return this.$store.state.files.modalPath;
		},
		size() {
			return prettyBytes(
				this.$store.state.files.files.find(
					(file) => file.Key === this.file.Key
				).Size
			);
		},
		uploadDate() {
			return this.file.LastModified.toLocaleString().split(",")[0];
		},
		extension() {
			return this.filePath.split(".").pop();
		},
		preSignedUrl() {
			return this.$store.getters["files/preSignedUrl"](this.filePath);
		},
		previewIsImage() {
			return ["bmp", "svg", "jpg", "jpeg", "png", "ico", "gif"].includes(
				this.extension
			);
		},
		previewIsVideo() {
			return ["m4v", "mp4", "webm", "mov", "mkv"].includes(
				this.extension
			);
		},
		previewIsAudio() {
			return ["mp3", "wav", "ogg"].includes(this.extension);
		},
		placeHolderDisplayable() {
			return [
				this.previewIsImage,
				this.previewIsVideo,
				this.previewIsAudio
			].every((value) => !!value === false);
		},
		objectMapUrlExists() {
			return this.objectMapUrl !== null;
		}
	},
	methods: {
		async getObjectMapUrl() {
			this.objectMapIsLoading = true;
			const objectMapUrl = await this.$store.state.files.getObjectMapUrl(
				this.filePath
			);

			await new Promise((resolve) => {
				const preload = new Image();
				preload.onload = resolve;
				preload.src = objectMapUrl;
			});

			this.objectMapUrl = objectMapUrl;
			this.objectMapIsLoading = false;
		},

		download() {
			this.$store.dispatch("files/download", this.file);
		},

		closeModal() {
			this.$store.commit("files/closeModal");
		},

		async copy() {
			await navigator.clipboard.writeText(this.objectLink);
			this.copyText = "Copied!";
			setTimeout(() => {
				this.copyText = "Copy Link";
			}, 2000);
		},

		async getSharedLink() {
			this.objectLink = await this.$store.state.files.getSharedLink(
				this.filePath
			);
		},

		stopClickPropagation(e) {
			if (e.target.id !== "detail-modal") {
				e.stopPropagation();
			}
		}
	},
	watch: {
		filePath() {
			this.getObjectMapUrl();
		}
	},
	created() {
		this.getObjectMapUrl();
	},
	components: {
		FileShareModal
	}
};
</script>
