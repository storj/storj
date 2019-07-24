// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class='delete-account-container'>
        <div class='delete-account' id="deleteAccountPopup">
            <div class='delete-account__info-panel-container'>
                <h2 class='delete-account__info-panel-container__main-label-text'>Delete account</h2>
                <svg width="240" height="187" viewBox="0 0 380 295" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path d="M168 295C246.997 295 311 231.2 311 152.5C311 73.8 246.997 10 168 10C89.0028 10 25 73.8 25 152.5C25 231.2 89.0028 295 168 295Z" fill="#E8EAF2"/>
                    <path fill-rule="evenodd" clip-rule="evenodd" d="M23.3168 98C21.4071 98 20 96.5077 20 94.6174C20.9046 68.9496 31.8599 45.769 49.0467 28.7566C66.2335 11.7442 89.6518 0.900089 115.583 0.00470057C117.492 -0.094787 119 1.39753 119 3.28779V32.4377C119 34.2284 117.593 35.6213 115.784 35.7208C99.7025 36.5167 85.2294 43.3813 74.4751 53.927C63.8213 64.5722 56.8863 78.8984 56.0822 94.8164C55.9817 96.6072 54.5746 98 52.7655 98H23.3168Z" fill="#B0B6C9"/>
                    <path d="M117.5 30C124.404 30 130 25.0751 130 19C130 12.9249 124.404 8 117.5 8C110.596 8 105 12.9249 105 19C105 25.0751 110.596 30 117.5 30Z" fill="#8F96AD"/>
                    <path d="M112.5 97C116.09 97 119 94.3137 119 91C119 87.6863 116.09 85 112.5 85C108.91 85 106 87.6863 106 91C106 94.3137 108.91 97 112.5 97Z" fill="#B0B6C9"/>
                    <path d="M15.0005 282C23.226 282 30 274.575 30 265.5C30 256.425 23.226 249 15.0005 249C6.77499 249 0.00102409 256.425 0.00102409 265.5C-0.0957468 274.678 6.67822 282 15.0005 282Z" fill="#8F96AD"/>
                    <path d="M15.5 274C19.0286 274 22 270.9 22 267C22 263.2 19.1214 260 15.5 260C11.9714 260 9 263.1 9 267C9 270.9 11.8786 274 15.5 274Z" fill="white"/>
                    <path d="M282.587 111H307.413C309.906 111 312 108.955 312 106.5C312 104.045 309.906 102 307.413 102H282.587C280.094 102 278 104.045 278 106.5C278 108.955 280.094 111 282.587 111Z" fill="white"/>
                    <path d="M282.585 93H289.415C291.951 93 294 91.02 294 88.5C294 85.98 291.951 84 289.415 84H282.585C280.049 84 278 85.98 278 88.5C278 91.02 279.951 93 282.585 93Z" fill="#E8EAF2"/>
                    <path d="M252.872 92H260.128C262.823 92 265 90.4091 265 88.5C265 86.5909 262.823 85 260.128 85H252.872C250.177 85 248 86.5909 248 88.5C248 90.4091 250.177 92 252.872 92Z" fill="#363840"/>
                    <path fill-rule="evenodd" clip-rule="evenodd" d="M45 166C48.8182 166 52 162.818 52 159C52 155.182 48.8182 152 45 152C41.1818 152 38 155.182 38 159C38 162.818 41.1818 166 45 166Z" fill="#B0B6C9"/>
                    <path fill-rule="evenodd" clip-rule="evenodd" d="M217 232C220.818 232 224 228.818 224 225C224 221.182 220.818 218 217 218C213.182 218 210 221.182 210 225C210 228.818 213.182 232 217 232Z" fill="#2683FF"/>
                    <path fill-rule="evenodd" clip-rule="evenodd" d="M26 142C29.8182 142 33 139.045 33 135.5C33 131.955 29.8182 129 26 129C22.1818 129 19 131.955 19 135.5C19 139.045 22.1818 142 26 142Z" fill="white"/>
                    <path fill-rule="evenodd" clip-rule="evenodd" d="M45 142C48.8182 142 52 139.045 52 135.5C52 131.955 48.8182 129 45 129C41.1818 129 38 131.955 38 135.5C38 139.045 41.1818 142 45 142Z" fill="#E8EAF2"/>
                    <path fill-rule="evenodd" clip-rule="evenodd" d="M64 142C67.8182 142 71 139.045 71 135.5C71 131.955 67.8182 129 64 129C60.1818 129 57 131.955 57 135.5C57 139.045 60.1818 142 64 142Z" fill="white"/>
                    <path d="M107.014 129.651C107.014 129.651 152.017 118.395 199.527 125.169C212.857 127.061 224.785 134.831 232.001 146.186C245.031 166.606 263.374 203.062 259.465 241.112L239.018 246.093C239.018 246.093 224.885 200.97 209.049 182.643C209.049 182.643 190.205 225.275 191.208 248.683C191.208 249.38 191.308 249.977 191.308 250.575C193.513 273.485 101 254.858 101 254.858L107.014 129.651Z" fill="#F5F6FA"/>
                    <path d="M143 89.7894L145.01 121.569C145.211 124.568 147.12 127.066 149.833 127.865C156.063 129.664 167.821 131.863 179.276 127.266C181.387 126.466 182.492 123.968 181.789 121.669L166.514 73L143 89.7894Z" fill="#8F96AD"/>
                    <path d="M189 61.014C189 61.014 186.474 85.2772 181.219 95.8484C175.964 106.42 174.448 114.272 161.412 109.641C148.376 105.01 141.707 93.5328 142.01 80.2434C142.01 80.2434 142.414 59.7052 147.972 54.3692C153.631 49.0333 189 61.014 189 61.014Z" fill="#B0B6C9"/>
                    <path d="M150.596 75.686L152.115 76.4754C152.115 76.4754 153.128 60.6872 159.814 61.4766C166.5 62.266 190.609 69.8641 199.625 64.9303C208.235 60.1938 191.521 44.2082 180.074 40.4585C163.866 35.0313 150.798 35.5247 144.822 45.2936C144.416 45.8857 143.606 45.8857 143.201 45.2936C142.492 44.0108 128.209 53.9772 132.97 65.917C133.172 66.5091 138.946 83.4815 140.567 83.9748C140.972 84.0735 141.479 83.8762 141.681 83.4815L146.24 74.4032C146.442 73.9098 147.05 73.7125 147.557 74.0085L150.596 75.686Z" fill="#0F002D"/>
                    <path d="M149.877 78.0283C149.877 78.0283 154.31 62.6808 145.56 63.0051C136.81 63.3293 139.844 79.7576 144.744 83L149.877 78.0283Z" fill="#B0B6C9"/>
                    <path d="M106.635 221.07C104.63 206.983 119.272 186.154 125.289 178.305C126.994 176.092 127.996 173.274 127.996 170.457C128.197 150.433 119.773 137.553 106.335 129C106.335 129 57.5953 185.953 70.0308 229.724C71.3345 234.453 73.4406 238.478 76.048 242C78.0538 225.397 97.1082 221.875 106.635 221.07Z" fill="#F5F6FA"/>
                    <path d="M107.966 215L106 214.798C107.655 200.851 120.172 183.67 125.448 177L127 178.112C121.828 184.681 109.621 201.559 107.966 215Z" fill="#0F002D"/>
                    <path d="M107.128 221.954C106.926 221.337 106.825 220.617 106.725 220C97.054 220.823 78.0147 224.423 76 241.29C97.8599 270.808 158 260.111 158 260.111V248.592C158.101 248.695 111.862 239.953 107.128 221.954Z" fill="#B0B6C9"/>
                    <path d="M152 257C152 257 160.863 236.189 176.575 243.593C192.187 250.997 190.978 255.799 190.978 255.799L152 257Z" fill="#B0B6C9"/>
                    <path d="M271.213 238H136.787C134.194 238 132 235.787 132 233.172V139.828C132 137.213 134.194 135 136.787 135H271.213C273.806 135 276 137.213 276 139.828V233.172C276 235.787 273.906 238 271.213 238Z" fill="#363840"/>
                    <path d="M217.252 258H195.744C193.109 258 191 256 191 253.5V190.5C191 188 193.109 186 195.744 186H217.252C219.888 186 221.996 188 221.996 190.5V253.5C222.102 255.9 219.888 258 217.252 258Z" fill="#363840"/>
                    <path d="M246.189 254H150.811C149.305 254 148 255.444 148 257.111V258.889C148 260.556 149.305 262 150.811 262H246.189C247.695 262 249 260.556 249 258.889V257.111C249 255.444 247.795 254 246.189 254Z" fill="#363840"/>
                    <path d="M350.452 224.555C349.952 224.555 349.553 224.555 349.154 224.654C348.355 224.754 347.557 224.256 347.257 223.56C337.873 206.543 319.705 195 298.742 195C279.775 195 263.004 204.454 253.121 218.883C252.622 219.579 251.724 219.878 250.925 219.778C248.429 219.281 245.834 218.982 243.239 218.982C223.772 219.082 208 234.605 208 253.91C208 253.91 208 253.91 208 254.01C208 255.104 208.898 256 210.096 256H377.904C379.002 256 380 255.104 380 254.01V253.91C379.8 237.591 366.623 224.555 350.452 224.555Z" fill="#B0B6C9"/>
                    <path d="M206 195C210.418 195 214 191.194 214 186.5C214 181.806 210.418 178 206 178C201.582 178 198 181.806 198 186.5C198 191.194 201.582 195 206 195Z" fill="white"/>
                </svg>
            </div>
            <div class='delete-account__form-container'>
                <p>Are you sure you want to delete your account? If you do so, all your information, projects and API Keys will be deleted forever.(drop from the satellite)</p>
                <HeaderedInput 
                    label='Enter your password' 
                    placeholder='Your Password'
                    class='full-input'
                    width='100%'
                    isPassword
                    :error='passwordError'
                    @setData='setPassword'>
                </HeaderedInput>
                <div class='delete-account__form-container__button-container'>
                    <Button label='Cancel' width='205px' height='48px' :onPress='onCloseClick' isWhite="true"/>
                    <Button label='Delete' width='205px' height='48px' class='red' :onPress='onDeleteAccountClick'/>
                </div>
            </div>
            <div class='delete-account__close-cross-container' @click='onCloseClick'>
                <svg width='16' height='16' viewBox='0 0 16 16' fill='none' xmlns='http://www.w3.org/2000/svg'>
                    <path d='M15.7071 1.70711C16.0976 1.31658 16.0976 0.683417 15.7071 0.292893C15.3166 -0.0976311 14.6834 -0.0976311 14.2929 0.292893L15.7071 1.70711ZM0.292893 14.2929C-0.0976311 14.6834 -0.0976311 15.3166 0.292893 15.7071C0.683417 16.0976 1.31658 16.0976 1.70711 15.7071L0.292893 14.2929ZM1.70711 0.292893C1.31658 -0.0976311 0.683417 -0.0976311 0.292893 0.292893C-0.0976311 0.683417 -0.0976311 1.31658 0.292893 1.70711L1.70711 0.292893ZM14.2929 15.7071C14.6834 16.0976 15.3166 16.0976 15.7071 15.7071C16.0976 15.3166 16.0976 14.6834 15.7071 14.2929L14.2929 15.7071ZM14.2929 0.292893L0.292893 14.2929L1.70711 15.7071L15.7071 1.70711L14.2929 0.292893ZM0.292893 1.70711L14.2929 15.7071L15.7071 14.2929L1.70711 0.292893L0.292893 1.70711Z' fill='#384B65'/>
                </svg>
            </div>
        </div>
    </div>
</template>

<script lang='ts'>
    import { Component, Vue } from 'vue-property-decorator';
    import HeaderedInput from '@/components/common/HeaderedInput.vue';
    import Button from '@/components/common/Button.vue';
    import { AuthToken } from '@/utils/authToken';
    import { APP_STATE_ACTIONS, USER_ACTIONS, NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';
    import { RequestResponse } from '@/types/response';

    @Component({
        components: {
            HeaderedInput,
            Button
        }
    })
    export default class DeleteAccountPopup extends Vue {
        public passwordError: string = '';
        private password: string = '';
        private isLoading: boolean = false;

        public setPassword(value: string): void {
            this.password = value;
        }

        public async onDeleteAccountClick(): Promise<void> {
            if (this.isLoading) {
                return;
            }

            this.isLoading = true;

            let response: RequestResponse<object> = await this.$store.dispatch(USER_ACTIONS.DELETE, this.password);

            if (!response.isSuccess) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, response.errorMessage);
                this.isLoading = false;

                return;
            }

            this.$store.dispatch(NOTIFICATION_ACTIONS.SUCCESS, 'Account was successfully deleted');
            AuthToken.remove();

            this.isLoading = false;
            this.$router.push('/login');
        }

        public onCloseClick(): void {
            this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_DEL_ACCOUNT);
        }
    }
</script>

<style scoped lang='scss'>
    .delete-account-container {
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

    .text {
        margin: 0;
        margin-bottom: 0 !important;
        font-family: 'font_regular' !important;
        font-size: 16px;
        line-height: 25px;
    }

    .delete-account {
        width: 100%;
        max-width: 845px;
        background-color: #FFFFFF;
        border-radius: 6px;
        display: flex;
        flex-direction: row;
        align-items: flex-start;
        position: relative;
        justify-content: center;
        padding: 100px 100px 100px 80px;

        &__info-panel-container {
            display: flex;
            flex-direction: column;
            justify-content: flex-start;
            align-items: center;
            margin-right: 100px;

            &__main-label-text {
                font-family: 'font_bold';
                font-size: 32px;
                line-height: 39px;
                color: #384B65;
                margin-bottom: 60px;
                margin-top: 0;
            }
        }

        &__form-container {
            width: 100%;
            max-width: 450px;

            p {
                margin: 0;
                margin-bottom: 25px;
                font-family: 'font_medium';
                font-size: 16px;
                line-height: 25px;

                &:nth-child(2) {
                    margin-top: 20px;
                }
            }

            a {
                font-family: 'font_medium';
                font-size: 16px;
                color: #2683FF;
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
        .delete-account {
            padding: 10px;

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
