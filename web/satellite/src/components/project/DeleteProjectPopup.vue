// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="delete-project-popup-container">
        <div class="delete-project-popup" id="deleteProjectPopup">
            <div class="delete-project-popup__info-panel-container">
                <h2 class="delete-project-popup__info-panel-container__main-label-text">Delete Project</h2>
                <svg width="245" height="245" viewBox="0 0 245 245" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path d="M117.664 202.065C173.34 202.065 218.448 156.815 218.448 100.997C218.448 45.1791 173.34 0 117.664 0C61.9881 0 16.8799 45.25 16.8799 101.068C16.8799 156.886 61.9881 202.065 117.664 202.065Z" fill="#E8EAF2"/>
                    <path d="M78.4429 45.676C78.1592 45.676 77.9464 45.676 77.7336 45.676C77.3081 45.676 76.8825 45.4632 76.6697 45.1086C71.5632 35.7465 61.4918 29.3633 49.9311 29.3633C39.5051 29.3633 30.2849 34.6117 24.8237 42.5553C24.54 42.9808 24.0435 43.1227 23.618 43.0518C22.2704 42.7681 20.8519 42.6262 19.3625 42.6262C8.65283 42.5553 0 51.1372 0 61.7759C0 62.4142 0.496474 62.9107 1.1348 62.9107H93.4789C94.1172 62.9107 94.6137 62.4142 94.6137 61.7759C94.6137 52.9103 87.3794 45.676 78.4429 45.676Z" fill="#B0B6C9"/>
                    <path d="M24.7535 199.866C30.6403 198.447 34.2575 192.348 32.839 186.248C31.4205 180.149 25.3919 176.39 19.5051 177.737C13.6183 179.085 9.93026 185.326 11.4197 191.355C12.7673 197.525 18.7959 201.284 24.7535 199.866Z" fill="#8F96AD"/>
                    <path d="M23.2649 193.625C25.8892 192.987 27.5914 190.362 26.953 187.667C26.3147 185.043 23.6905 183.341 20.9953 183.979C18.3711 184.617 16.6689 187.242 17.3073 189.937C17.8747 192.703 20.5698 194.263 23.2649 193.625Z" fill="#F5F6FA"/>
                    <path d="M160.005 182.063C155.679 176.743 143.834 165.963 143.834 165.963L139.934 167.24C139.934 167.24 147.948 187.098 151.636 188.446C155.324 189.794 159.083 187.169 160.36 184.545C160.715 183.765 160.573 182.772 160.005 182.063Z" fill="#363840"/>
                    <path d="M84.8965 149.721L95.6061 150.359C95.6061 150.359 130.714 116.174 152.488 113.833C152.488 113.833 166.247 133.905 148.232 168.658C146.743 171.495 143.126 172.559 140.572 170.644C139.792 170.076 139.296 169.367 139.012 168.516C140.218 157.026 127.026 149.792 117.947 156.884C113.196 160.643 109.082 163.835 107.522 165.254C102.273 169.722 77.7331 165.324 84.8965 149.721Z" fill="#8F96AD"/>
                    <path d="M179.014 237.455C174.688 232.136 162.843 221.355 162.843 221.355L158.942 222.632C158.942 222.632 166.957 242.491 170.645 243.839C174.333 245.186 178.092 242.562 179.369 239.938C179.794 239.158 179.652 238.165 179.014 237.455Z" fill="#363840"/>
                    <path d="M84.7549 157.523L134.828 161.212C134.828 161.212 161.921 186.674 164.829 223.058C164.829 223.058 165.538 225.753 160.29 225.895C139.58 190.149 112.486 182.347 95.7482 181.425C83.2655 180.716 71.5629 175.113 63.4065 165.68C62.981 165.112 62.6973 164.829 62.6973 164.829L84.7549 157.523Z" fill="#B0B6C9"/>
                    <path d="M88.5139 117.237C88.5139 117.237 92.2019 144.614 117.948 156.955L127.381 156.459L129.367 160.147L107.309 159.438C107.309 159.438 88.9394 150.785 80.2156 129.862C79.9319 129.153 75.18 117.45 88.5139 117.237Z" fill="#8F96AD"/>
                    <path d="M88.8686 157.31C88.8686 157.31 83.762 140.288 83.4783 123.975C83.4783 123.975 88.2303 120.429 91.1382 118.94C92.0602 118.443 92.4858 117.308 92.0602 116.386C90.2871 111.989 78.8682 94.4704 77.9462 94.1158C72.9815 92.5555 52.7679 81.3494 36.8098 100.428C30.001 108.655 35.8168 135.607 62.6974 164.828L65.1797 167.807L88.8686 157.31Z" fill="#F5F6FA"/>
                    <path d="M67.0949 126.6C67.0949 126.6 82.8402 147.806 104.472 152.133C104.472 152.133 110.43 147.381 114.473 149.437C118.515 151.565 123.764 160.005 123.764 160.005L95.8194 158.232C95.8194 158.232 69.4354 155.608 57.2363 138.799C58.6548 133.267 62.0592 130.146 67.0949 126.6Z" fill="#B0B6C9"/>
                    <path d="M60.1432 78.7959L55.604 90.995C55.3912 91.5624 55.604 92.2007 56.0295 92.5553C57.7317 93.9029 62.2709 96.669 68.8669 94.6831C69.6471 94.4703 70.1436 93.832 70.2854 93.0518L71.8458 82.2712L60.1432 78.7959Z" fill="#8F96AD"/>
                    <path d="M82.5562 70.0719C82.5562 70.0719 79.932 82.8384 74.6836 86.5265C69.5061 90.1436 60.0021 86.5974 57.3779 76.6679L64.896 60.4971L82.5562 70.0719Z" fill="#B0B6C9"/>
                    <path d="M63.5478 71.1363C63.9025 71.2782 64.328 71.0654 64.3989 70.7108C64.6826 69.505 65.8884 63.9729 70.4985 66.8099C75.1086 69.6469 81.9883 73.4768 86.5275 69.2213C101.067 55.4619 78.0165 56.8095 71.8461 53.476C70.1439 52.554 61.7747 48.5822 58.1576 55.4619C57.9448 55.8875 52.4836 69.0086 57.2355 77.5905C57.4483 78.016 58.0866 77.8742 58.2994 77.4486L61.491 70.9945C61.6329 70.7108 61.9875 70.5689 62.2712 70.7108L63.5478 71.1363Z" fill="#363840"/>
                    <path d="M63.5477 72.4828C63.5477 72.4828 62.2711 65.4613 59.5759 67.589C56.8808 69.6459 59.7887 74.8234 62.6966 75.8872L63.5477 72.4828Z" fill="#B0B6C9"/>
                    <path d="M122.274 161.495L137.452 120.429C138.091 118.727 139.651 117.663 141.424 117.592L158.446 117.521C160.148 117.521 161.354 119.295 160.645 120.855L144.119 159.935C143.552 161.211 142.346 162.062 140.999 162.062L124.189 162.842L122.274 161.495Z" fill="#363840"/>
                    <path d="M145.324 140.748C146.38 138.915 146.41 136.954 145.392 136.367C144.374 135.781 142.692 136.791 141.637 138.624C140.581 140.457 140.55 142.418 141.568 143.005C142.587 143.591 144.268 142.581 145.324 140.748Z" fill="#F7F7FF"/>
                    <path d="M124.261 162.984H124.19L88.7276 158.941C87.8765 158.87 87.2382 158.019 87.3091 157.168C87.38 156.317 88.2311 155.679 89.0822 155.75L124.545 159.792C125.396 159.863 126.034 160.714 125.963 161.565C125.821 162.416 125.112 162.984 124.261 162.984Z" fill="#363840"/>
                    <path d="M83.4782 124.401C83.2655 124.401 83.1236 124.188 83.1236 123.975C83.6201 116.528 81.3505 106.528 81.3505 106.457C81.2796 106.244 81.4214 106.031 81.6342 106.031C81.847 105.96 82.0597 106.102 82.0597 106.315C82.0597 106.386 84.3293 116.528 83.8328 124.046C83.8328 124.259 83.691 124.401 83.4782 124.401Z" fill="#0F002D"/>
                    <path d="M67.0946 127.025C66.9527 127.025 66.8818 126.954 66.8109 126.883C66.7399 126.812 59.5765 116.386 57.307 109.435C57.236 109.222 57.3779 109.01 57.5197 108.939C57.7325 108.868 57.9453 109.01 58.0162 109.152C60.2149 116.031 67.3074 126.315 67.3783 126.386C67.5201 126.528 67.4492 126.812 67.3074 126.883C67.2364 127.025 67.1655 127.025 67.0946 127.025Z" fill="#0F002D"/>
                    <path d="M55.1802 141.493L54.8965 141.351C43.9031 135.819 37.8036 121.918 37.7327 121.776C37.6617 121.563 37.7327 121.35 37.9454 121.28C38.1582 121.209 38.371 121.28 38.4419 121.492C38.5128 121.634 44.3996 135.039 54.9674 140.571L56.1731 139.082C56.315 138.94 56.5277 138.869 56.6696 139.011C56.8114 139.153 56.8824 139.365 56.7405 139.507L55.1802 141.493Z" fill="#0F002D"/>
                    <path d="M181.001 77.4491C196.746 71.1368 215.967 68.8672 215.967 68.8672C231.215 94.6838 239.514 128.018 239.514 128.018C228.449 135.962 212.633 136.033 212.633 136.033C206.179 125.252 184.263 96.4569 184.263 96.4569C176.107 98.4428 168.872 98.6556 162.489 97.7336C164.759 91.5631 164.333 84.8962 163.837 81.0663C170.078 80.6407 176.107 79.3641 181.001 77.4491Z" fill="white"/>
                    <path d="M125.183 71.9885C126.034 69.5061 128.303 69.648 128.303 69.648C134.403 78.6554 149.723 81.918 163.766 81.0669C164.262 84.8968 164.617 91.4928 162.418 97.7342C137.098 93.9043 125.183 71.9885 125.183 71.9885Z" fill="#B0B6C9"/>
                    <path d="M125.75 72.3437C125.75 72.9111 127.949 72.982 128.303 70.4287C127.949 69.3648 126.885 64.8256 125.963 64.1873C121.069 60.9957 112.629 57.875 112.629 57.875C111.991 59.2935 113.267 60.0027 113.267 60.0027L117.523 61.9886C117.31 63.4071 116.955 64.6838 117.097 66.1732C117.239 67.308 117.806 68.3719 118.728 69.152C119.65 69.9322 120.856 70.8542 121.92 71.5635C123.906 72.982 125.75 72.3437 125.75 72.3437Z" fill="#8F96AD"/><path d="M119.65 63.6191L123.338 64.8958L125.111 66.8817L123.692 70.5698C123.692 70.5698 121.068 66.2434 120.075 65.4632C119.011 64.683 119.153 64.1156 119.65 63.6191Z" fill="#8F96AD"/>
                    <path d="M192.136 58.4424L202.774 71.1379C202.916 71.3507 202.987 71.6344 202.774 71.8472C201.711 73.1238 197.597 77.2375 190.15 75.748C189.37 75.6062 188.731 75.1097 188.306 74.4005L181.852 62.4142L192.136 58.4424Z" fill="#8F96AD"/>
                    <path d="M175.965 49.0805C175.965 49.0805 174.404 61.6342 176.745 65.606C179.085 69.5778 185.823 66.5989 190.504 60.9249L190.362 48.0166L175.965 49.0805Z" fill="#B0B6C9"/>
                    <path d="M188.164 55.4632C187.88 55.4632 187.667 55.1795 187.738 54.8958C187.951 54.0447 188.235 52.3425 188.022 51.0659C188.022 51.0659 188.802 50.0729 184.901 50.5694C181.071 51.0659 176.887 51.6333 174.617 49.08C172.418 46.5267 172.844 40.569 175.61 38.9377C180.929 35.817 197.809 43.406 197.88 46.243C197.951 49.1509 196.32 53.761 196.249 53.9029C195.824 55.0377 193.767 59.7896 190.362 61.6337C190.079 61.7755 189.724 61.5628 189.653 61.2081L189.511 56.0306C189.511 55.8178 189.299 55.6051 189.086 55.6051L188.164 55.4632Z" fill="#363840"/>
                    <path d="M187.739 56.3856C187.739 56.3856 190.647 52.2011 191.781 54.3288C192.916 56.5275 189.512 59.0099 187.242 58.868L187.739 56.3856Z" fill="#B0B6C9"/>
                    <path d="M215.753 86.5284L208.235 78.7976L215.966 68.8682C215.966 68.8682 234.619 73.2655 241.925 85.1099C239.229 86.3866 234.407 89.0817 230.364 93.4791C226.959 87.876 215.753 86.5284 215.753 86.5284Z" fill="white"/>
                    <path d="M208.306 105.961C208.306 105.961 228.52 109.011 231.073 98.7979C231.57 96.6701 231.215 94.897 230.364 93.4785C234.407 89.0812 239.23 86.386 241.925 85.1094C244.123 88.6556 245.329 92.9111 244.62 97.9468C241.641 119.721 207.668 110.926 207.668 110.926L208.306 105.961Z" fill="#8F96AD"/>
                    <path d="M209.725 110.288C209.725 110.288 209.016 104.614 207.81 102.983C206.604 101.352 204.689 99.3659 201.568 101.139C198.448 102.912 195.114 105.82 195.114 105.82C195.114 105.82 196.249 108.161 198.448 106.387L197.384 108.161C197.384 108.161 198.377 110.005 200.079 108.161L199.228 110.288C199.228 110.288 200.717 111.494 201.923 109.934C203.058 108.302 205.54 112.558 209.725 110.288Z" fill="#B0B6C9"/>
                    <path d="M111.991 28.2267H108.515C106.742 28.2267 105.253 26.8082 105.253 25.106C105.253 23.4038 106.742 21.9854 108.515 21.9854H111.991C113.764 21.9854 115.253 23.4038 115.253 25.106C115.253 26.8082 113.764 28.2267 111.991 28.2267Z" fill="white"/>
                    <path d="M97.2377 15.674H65.3924C63.6193 15.674 62.1299 14.2555 62.1299 12.5533C62.1299 10.8511 63.6193 9.43262 65.3924 9.43262H97.2377C99.0108 9.43262 100.5 10.8511 100.5 12.5533C100.5 14.2555 99.0108 15.674 97.2377 15.674Z" fill="#F5F6FA"/>
                    <path d="M130.785 15.673H125.82C123.976 15.673 122.486 14.2545 122.486 12.5523C122.486 10.8501 123.976 9.43164 125.82 9.43164H130.785C132.629 9.43164 134.118 10.8501 134.118 12.5523C134.118 14.2545 132.629 15.673 130.785 15.673Z" fill="#363840"/>
                    <path fill-rule="evenodd" clip-rule="evenodd" d="M184.334 148.02C186.887 148.02 189.015 145.893 189.015 143.339C189.015 140.786 186.887 138.658 184.334 138.658C181.781 138.658 179.653 140.786 179.653 143.339C179.582 145.893 181.71 148.02 184.334 148.02Z" fill="#A5A9B5"/>
                </svg>
            </div>
            <div class="delete-project-popup__form-container">
                <p>Are you sure that you want to delete your project? You will lose all your buckets and files that linked to this project.</p>
                <div>
                    <p class="text" v-if="!nameError">To confirm, enter the project name</p>
                    <div v-if="nameError" class="delete-project-popup__form-container__label">
                        <img src="../../../static/images/register/ErrorInfo.svg"/>
                        <p class="text">{{nameError}}</p>
                    </div>
                    <input 
                        type="text" 
                        placeholder="Enter Project Name"
                        v-model="projectName"
                        v-on:keyup="resetError" >
                </div>
                <div class="delete-project-popup__form-container__button-container">
                    <Button label="Cancel" width="205px" height="48px" :onPress="onCloseClick" isWhite="true"/>
                    <Button 
                        label="Delete"
                        width="205px" 
                        height="48px" 
                        class="red"
                        :onPress="onDeleteProjectClick" 
                        :isDisabled="isDeleteButtonDisabled" />
                </div>
            </div>
            <div class="delete-project-popup__close-cross-container" @click="onCloseClick">
                <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path d="M15.7071 1.70711C16.0976 1.31658 16.0976 0.683417 15.7071 0.292893C15.3166 -0.0976311 14.6834 -0.0976311 14.2929 0.292893L15.7071 1.70711ZM0.292893 14.2929C-0.0976311 14.6834 -0.0976311 15.3166 0.292893 15.7071C0.683417 16.0976 1.31658 16.0976 1.70711 15.7071L0.292893 14.2929ZM1.70711 0.292893C1.31658 -0.0976311 0.683417 -0.0976311 0.292893 0.292893C-0.0976311 0.683417 -0.0976311 1.31658 0.292893 1.70711L1.70711 0.292893ZM14.2929 15.7071C14.6834 16.0976 15.3166 16.0976 15.7071 15.7071C16.0976 15.3166 16.0976 14.6834 15.7071 14.2929L14.2929 15.7071ZM14.2929 0.292893L0.292893 14.2929L1.70711 15.7071L15.7071 1.70711L14.2929 0.292893ZM0.292893 1.70711L14.2929 15.7071L15.7071 14.2929L1.70711 0.292893L0.292893 1.70711Z" fill="#384B65"/>
                </svg>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import Button from '@/components/common/Button.vue';
    import { EMPTY_STATE_IMAGES } from '@/utils/constants/emptyStatesImages';
    import { PROJETS_ACTIONS, NOTIFICATION_ACTIONS, PM_ACTIONS, APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
    import { API_KEYS_ACTIONS } from '@/utils/constants/actionNames';

    @Component({
        components: {
            Button
        }
    })
    export default class DeleteProjectPopup extends Vue {
        private projectName: string = '';
        private nameError: string = '';
        private isLoading: boolean = false;

        public resetError (): void {
            this.nameError = '';
        }

        public async onDeleteProjectClick(): Promise<void> {
            if (this.isLoading) {
                return;
            }

            this.isLoading = true;

            if (!this.validateProjectName()) {
                return;
            }

            if (!await this.deleteProject()) {
                return;
            }

            this.$store.dispatch(PM_ACTIONS.CLEAR);
            this.$store.dispatch(NOTIFICATION_ACTIONS.SUCCESS, 'Project was successfully deleted');
            this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_DEL_PROJ);

            this.selectProject();

            this.isLoading = false;
        }

        public  onCloseClick(): void {
            this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_DEL_PROJ);
        }

        public get isDeleteButtonDisabled(): boolean {
            return !this.projectName || !!this.nameError;
        }

        private validateProjectName(): boolean {
            if (this.projectName === this.$store.getters.selectedProject.name) {
                return true;
            }

            this.nameError = 'Name doesn\'t match with current project name';
            this.isLoading = false;

            return false;
        }

        private async deleteProject(): Promise<boolean> {
            let response = await this.$store.dispatch(PROJETS_ACTIONS.DELETE, this.$store.getters.selectedProject.id);
            if (!response.isSuccess) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, response.errorMessage);
                this.isLoading = false;
            }

            return response.isSuccess;
        }

        private selectProject(): void {
            if (this.$store.state.projectsModule.projects.length === 0) {
                return;
            }

            this.$store.dispatch(PROJETS_ACTIONS.SELECT, this.$store.state.projectsModule.projects[0].id);
            this.$store.dispatch(PM_ACTIONS.FETCH);
            this.$store.dispatch(API_KEYS_ACTIONS.FETCH);
        }
    }
</script>

<style scoped lang="scss">
    .delete-project-popup-container {
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        background-color: rgba(134, 134, 148, 0.4);
        z-index: 1121;
        display: flex;
        justify-content: center;
        align-items: center;
    }

    .input-container.full-input {
        width: 100%;
    }

    .red {
        background-color: #EB5757;
    }

    .delete-project-popup {
        width: 100%;
        max-width: 800px;
        height: 460px;
        background-color: #FFFFFF;
        border-radius: 6px;
        display: flex;
        flex-direction: row;
        align-items: center;
        position: relative;
        justify-content: space-between;
        padding: 20px 100px 0px 100px;

        &__info-panel-container {
            display: flex;
            flex-direction: column;
            justify-content: flex-start;
            align-items: center;
            margin-right: 55px;

            &__main-label-text {
                font-family: 'font_bold';
                font-size: 32px;
                line-height: 39px;
                color: #384B65;
                margin-bottom: 30px;
                margin-top: 0;
            }
        }

        &__form-container {
            width: 100%;
            max-width: 440px;
            height: 335px;

            p {
                font-family: 'font_medium';
                font-size: 16px;
                line-height: 21px;
                margin-bottom: 30px;
            }

            &__label {
                display: flex;
                flex-direction: row;
                align-items: center;

                p {
                    padding-left: 10px;
                    color: #EB5757;
                    margin: 0;
                }
            }

            .text {
                margin: 0px;
            }

            input {
                font-family: 'font_regular';
                font-size: 16px;
                line-height: 21px;
                margin-top: 10px;
                resize: none;
                margin-bottom: 18px;
                height: 48px;
                width: 100%;
                text-indent: 20px;
                border-color: rgba(56, 75, 101, 0.4);
                border-radius: 6px;
                outline: none;
                box-shadow: none;
            }

            &__button-container {
                width: 100%;
                display: flex;
                flex-direction: row;
                justify-content: space-between;
                align-items: center;
                margin-top: 30px;
            }
        }

        &__close-cross-container {
            display: flex;
            justify-content: center;
            align-items: center;
            position: absolute;
            right: 30px;
            top: 40px;
            height: 24px;
            width: 24px;
            cursor: pointer;

            &:hover svg path {
                fill: #2683FF;
            }
        }
    }

    @media screen and (max-width: 720px) {
        .delete-project-popup {

            &__info-panel-container {
                display: none;
            }

            &__form-container {

                &__button-container {
                    width: 100%;
                }
            }
        }
    }
</style>
