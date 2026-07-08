import { Layout } from '@/routers/constant';

const enhanceRouter = {
    sort: 13,
    path: '/enhance',
    name: 'Enhance-Menu',
    component: Layout,
    redirect: '/enhance/home',
    meta: {
        title: 'setting.enhance',
        icon: 'p-toolbox',
    },
    children: [
        {
            path: 'home',
            name: 'EnhanceHome',
            hidden: true,
            component: () => import('@/views/setting/enhance/index.vue'),
            meta: {
                title: 'setting.enhance',
                requiresAuth: true,
                activeMenu: '/enhance',
            },
        },
        {
            path: 'simple-node',
            name: 'EnhanceSimpleNode',
            hidden: true,
            component: () => import('@/views/setting/enhance/simple-node/index.vue'),
            meta: {
                title: 'xpack.node.nodeManagement',
                requiresAuth: true,
                activeMenu: '/enhance',
            },
        },
        {
            path: 'windows-service',
            name: 'EnhanceWindowsService',
            hidden: true,
            component: () => import('@/views/setting/enhance/windows-service/index.vue'),
            meta: {
                title: 'setting.enhanceWindowsServiceAction',
                requiresAuth: true,
                activeMenu: '/enhance',
            },
        },
        {
            path: 'local-app',
            name: 'EnhanceLocalApp',
            hidden: true,
            component: () => import('@/views/setting/enhance/local-app/index.vue'),
            meta: {
                title: 'setting.enhanceLocalAppTitle',
                requiresAuth: true,
                activeMenu: '/enhance',
            },
        },
        {
            path: '/settings/simple-node',
            name: 'SimpleNode',
            hidden: true,
            redirect: '/enhance/simple-node',
            meta: {
                requiresAuth: true,
                activeMenu: '/enhance',
                ignoreTab: true,
            },
        },
    ],
};

export default enhanceRouter;
