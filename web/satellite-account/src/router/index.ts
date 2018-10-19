import Vue from 'vue';
import Router from 'vue-router';
import ROUTES from '@/utils/constants/routerConstants';
import Login from '@/views/Login.vue';
import Register from '@/views/Register.vue';

Vue.use(Router);

export default new Router({
	mode: 'history',
	routes: [
		{
			path: ROUTES.DEFAULT.path,
			name: ROUTES.DEFAULT.name,
			component: Login
		},
		{
			path: ROUTES.REGISTER.path,
			name: ROUTES.REGISTER.name,
			component: Register
		}
  	]
});
