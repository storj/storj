<template>
  <div class="loginContainer">
	<img class="logo" src="../../static/images/login/Logo.svg" alt="logo" >
	<div class="loginArea">
		<div class="titleContainer">
			<h1>Welcome to Storj</h1>
		</div>
		<HeaderlessInput  
			class="emailInput"
			placeholder ="Email" 
			:error="emailError"
			@setData="setEmail"
			width="440px">
		</HeaderlessInput>
		<HeaderlessInput  
			class="passwordInput"
			placeholder ="Password" 
			:error="emailError"
			@setData="setEmail"
			width="440px"
			isPassword>
		</HeaderlessInput>
		<Button class="loginButton" label="Login" width="440px" height="48px" :onPress="onLogin"/>
		<!-- start of navigation area -->
			<div class="navigationArea">
				<router-link to="/register" class="navLink bold" exact><h3>Create account</h3></router-link>
				<router-link to="" class="navLink" exact><h3><strong>Forgot password</strong></h3></router-link>
			</div>
		<!-- end of navigation area -->
	</div>
  </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import HeaderlessInput from '@/components/common/HeaderlessInput.vue';
import Button from '@/components/common/Button.vue';

@Component({
	data: function() {

		return {
			email: '',
			emailError: '',
		}
	},
	methods: {
		setEmail: function(value : string) {

		}
	},
	components: {
		HeaderlessInput,
		Button
	}
})

export default class Home extends Vue {}
</script>

<style scoped lang="scss">
	.loginContainer {
		position: fixed;
		width: 100%;
		height: 100%;
		left: 0;
		top: 0;
		background: rgba(51,51,51,0.7);
		z-index: 10;
		background-image: url(../../static/images/login/Background.svg);
		background-repeat: no-repeat;
    	background-size: cover;
		display: flex;
		justify-content: flex-start;
		flex-direction: column;
		align-items: flex-start;
		padding: 60px 0px 190px 104px;
	}
	.titleContainer {
		width: 440px;
		height: 48px;
		display: flex;
		justify-content: flex-start;
		align-items: flex-start;
		margin-bottom: 32px;
	}
	h1{
		font-family: 'montserrat_bold';
		font-size: 32px;
		color: #384B65;
		line-height: 39px;
		margin-block-start: 0;
    	margin-block-end: 0;
	}
	.logo {
		width: 139px;
		height: 62px;
	}
	.loginArea {
		background-color: #fff;
		margin-top: 50px;
		width: 52.5vw;
		height: 60vh;
		border-radius: 6px;
		display: flex;
		justify-content: center;
		flex-direction: column;
		align-items: center;
	}
	.passwordInput {
		margin-top: 22px;
	}
	.loginButton {
		margin-top: 22px;
		align-self: center;
	}
	.navigationArea {
		margin-top: 24px;
		width: 440px;
		height: 48px;
		display: flex;
		justify-content: center;
		flex-direction: row;
		align-items: center;
	}
	.navLink {
		font-family: 'montserrat_regular';
		font-size: 14px;
		line-height: 20px;
		color: #2683FF;
		height: 48px;
		text-align: center;
		text-justify: center;
		padding-left: 15px;
		padding-right: 15px;
	}
	.navLink.bold {
		font-family: 'montserrat_medium';
	}
</style>


