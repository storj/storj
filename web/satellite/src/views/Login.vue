<template>
    <div class="login-container">
        <img class="login-container__logo" src="../../static/images/login/Logo.svg" alt="logo">
        <div class="login-area">
            <div class="login-area__title-container">
                <h1>Welcome to Storj</h1>
            </div>
            <HeaderlessInput
                    class="login-area__email-input"
                    placeholder="Email"
                    @setData="setEmail"
                    width="100%">
            </HeaderlessInput>
            <HeaderlessInput
                    class="login-area__password-input"
                    placeholder="Password"
                    @setData="setPassword"
                    width="100%"
                    isPassword>
            </HeaderlessInput>
            <Button class="login-area__login-button" label="Login" height="48px" :onPress="onLogin"/>
            <!-- start of navigation area -->
            <div class="login-area__navigation-area">
                <router-link to="/register" class="login-area__navigation-area__nav-link bold" exact><h3>Create
                    account</h3></router-link>
                <router-link to="" class="login-area__navigation-area__nav-link" exact><h3><strong>Forgot
                    password</strong></h3></router-link>
            </div>
            <!-- end of navigation area -->
        </div>
    </div>
</template>

<script lang="ts">
    import {Component, Vue} from 'vue-property-decorator';

    import HeaderlessInput from '@/components/common/HeaderlessInput.vue';
    import Button from '@/components/common/Button.vue';
    import {setToken} from "../utils/tokenManager";
    import ROUTES from "../utils/constants/routerConstants";
    import {login} from "../utils/qraphql/loginQuery";

    @Component({
        data: function () {

            return {
                email: '',
                password: '',
                token: ''
            }
        },
        methods: {
            setEmail: function (value: string) {
                this.$data.email = value;
            },
            setPassword: function (value: string) {
                this.$data.password = value;
            },
            onLogin: async function () {
                try {
                    let loginData = await login(this.$data.email, this.$data.password);

                    setToken(loginData.data.token.token);
                    this.$store.dispatch("setUserInfo", loginData.data.token.user)
                        .then(() => {
                            this.$router.push(ROUTES.DASHBOARD.path);
                        }).catch((error) => {
                        console.log(error);
                    });
                } catch (error) {
                   console.log(error)
                }
            }

        },
        computed: {},
        components: {
            HeaderlessInput,
            Button
        }
    })

    export default class Home extends Vue {
    }
</script>

<style scoped lang="scss">
    .login-container {
        position: fixed;
        width: 100%;
        height: 100%;
        left: 0;
        top: 0;
        background: rgba(51, 51, 51, 0.7);
        z-index: 10;
        background-image: url(../../static/images/login/Background.svg);
        background-repeat: no-repeat;
        background-size: cover;
        display: flex;
        justify-content: flex-start;
        flex-direction: column;
        align-items: flex-start;
        padding: 60px 0px 190px 104px;

        &__logo {
            width: 139px;
            height: 62px;
        }
    }

    .login-area {
        background-color: #fff;
        margin-top: 50px;
        max-width: 440px;
        width: 100%;
        padding: 120px;
        border-radius: 6px;
        display: flex;
        justify-content: center;
        flex-direction: column;
        align-items: flex-start;
        &__title-container {
            height: 48px;
            display: flex;
            justify-content: flex-start;
            align-items: flex-start;
            margin-bottom: 32px;
            h1 {
                font-family: 'montserrat_bold';
                font-size: 32px;
                color: #384B65;
                line-height: 39px;
                margin-block-start: 0;
                margin-block-end: 0;
            }
        }
        &__password-input {
            margin-top: 22px;
        }
        &__login-button {
            margin-top: 22px;
            align-self: center;
        }
        &__login-button.container {
            display: block;
            text-align: center;
        }
        &__navigation-area {
            margin-top: 24px;
            width: 100%;
            height: 48px;
            display: flex;
            justify-content: center;
            flex-direction: row;
            align-items: center;
            &__nav-link {
                font-family: 'montserrat_regular';
                font-size: 14px;
                line-height: 20px;
                color: #2683FF;
                height: 48px;
                text-align: center;
                text-justify: center;
                padding-left: 15px;
                padding-right: 15px;
                min-width: 140px;
                &:hover {
                    text-decoration: underline;
                }
                .bold {
                    font-family: 'montserrat_medium';
                }
            }
        }
    }

    @media screen and (max-width: 800px) {
        .login-container {
            padding: 0;
            justify-content: center;
            padding: 0 50px;
        }
        .login-area {
            max-width: 300px;
        }
    }
</style>
