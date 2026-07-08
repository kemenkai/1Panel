import router from '@/routers/router';
import NProgress from '@/config/nprogress';
import { GlobalStore } from '@/store';
import { AxiosCanceler } from '@/api/helper/axios-cancel';

const axiosCanceler = new AxiosCanceler();

let isRedirecting = false;
const shouldPinEnhanceHome = (path: string, activeMenu?: string) => activeMenu === '/enhance' && path === '/enhance';

const resolveValidCachedRoute = (cachedRoute: string, activeMenu?: string) => {
    if (!cachedRoute || !activeMenu) {
        return '';
    }
    const resolved = router.resolve(cachedRoute);
    if (!resolved.matched.length) {
        return '';
    }
    const resolvedActiveMenu = resolved.meta.activeMenu as string | undefined;
    if (resolvedActiveMenu && resolvedActiveMenu !== activeMenu) {
        return '';
    }
    return resolved.path;
};

router.beforeEach((to, from, next) => {
    NProgress.start();
    axiosCanceler.removeAllPending();
    const globalStore = GlobalStore();
    const isPublicRoute = to.name === 'entrance' || to.matched.some((record) => record.meta.requiresAuth === false);
    if (!isPublicRoute && !globalStore.isLogin) {
        next({
            name: 'entrance',
            params: to.params,
        });
        NProgress.done();
        return;
    }
    if (to.name === 'entrance' && globalStore.isLogin) {
        if (to.params.code === globalStore.entrance) {
            next({
                name: 'home',
            });
            NProgress.done();
            return;
        }
        next({ name: '404' });
        NProgress.done();
        return;
    }

    if (to.path === '/apps/all' && to.query.install != undefined) {
        return next();
    }
    const activeMenuKey = 'cachedRoute' + (to.meta.activeMenu || '');
    const pinEnhanceHome = shouldPinEnhanceHome(to.path, to.meta.activeMenu as string | undefined);
    if (to.query.uncached != undefined) {
        const query = { ...to.query };
        delete query.uncached;
        localStorage.removeItem(activeMenuKey);
        return next({ path: to.path, query });
    }

    const cachedRoute = resolveValidCachedRoute(
        localStorage.getItem(activeMenuKey) || '',
        to.meta.activeMenu as string | undefined,
    );
    if (pinEnhanceHome || !cachedRoute) {
        localStorage.removeItem(activeMenuKey);
    }
    if (
        to.meta.activeMenu &&
        !pinEnhanceHome &&
        to.path === to.meta.activeMenu &&
        to.meta.activeMenu != from.meta.activeMenu &&
        cachedRoute &&
        cachedRoute !== to.path &&
        !isRedirecting
    ) {
        isRedirecting = true;
        next(cachedRoute);
        NProgress.done();
        return;
    }

    if (!to.matched.some((record) => record.meta.requiresAuth)) return next();

    return next();
});

router.afterEach((to) => {
    if (to.meta.activeMenu && !to.meta.ignoreTab && !isRedirecting) {
        if (to.meta.activeMenu === '/enhance') {
            localStorage.removeItem('cachedRoute' + to.meta.activeMenu);
        } else {
            let notMathParam = true;
            if (to.matched.some((record) => record.path.includes(':'))) {
                notMathParam = false;
            }
            if (notMathParam) {
                if (to.meta.activeMenu === '/cronjobs' && to.path === '/cronjobs/cronjob/operate') {
                    localStorage.setItem('cachedRoute' + to.meta.activeMenu, '/cronjobs/cronjob');
                } else if (to.meta.activeMenu === '/containers' && to.path === '/containers/container/operate') {
                    localStorage.setItem('cachedRoute' + to.meta.activeMenu, '/containers/container');
                } else if (to.meta.activeMenu === '/toolbox' && to.path === '/toolbox/clam/setting') {
                    localStorage.setItem('cachedRoute' + to.meta.activeMenu, '/toolbox/clam');
                } else {
                    localStorage.setItem('cachedRoute' + to.meta.activeMenu, to.path);
                }
            }
        }
    }

    isRedirecting = false;
    NProgress.done();
});

export default router;
