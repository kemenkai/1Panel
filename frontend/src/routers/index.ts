import router from '@/routers/router';
import NProgress from '@/config/nprogress';
import { useGlobalStore } from '@/composables/useGlobalStore';
import { AxiosCanceler } from '@/api/helper/axios-cancel';
import { hasRouteAccess } from '@/utils/rbac';
import { loadProductProFromDB } from '@/utils/xpack';
import i18n from '@/lang';
import { MsgError } from '@/utils/message';

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

const enterpriseLicenseCheckWhiteList = ['EnterpriseLicenseRequired', 'entrance', 'login', 'Expired'];
const noLoginWhiteList = ['entrance', 'login', 'file-share', '404', 'Expired'];

const clearLicenseStatus = () => {
    const { isEnterpriseLicenseLoaded, isEnterpriseLicensed } = useGlobalStore();
    isEnterpriseLicensed.value = false;
    isEnterpriseLicenseLoaded.value = false;
};

const clearLoginStatus = () => {
    const { globalStore } = useGlobalStore();
    globalStore.setLogStatus(false);
    globalStore.clearAuthInfo();
    clearLicenseStatus();
};

router.beforeEach(async (to, from, next) => {
    const { entrance, isEnterprise, isEnterpriseLicenseLoaded, isEnterpriseLicensed, isLogin } = useGlobalStore();
    NProgress.start();
    axiosCanceler.removeAllPending();

    if (!isLogin.value) {
        clearLoginStatus();
    }
    if (!isLogin.value && !noLoginWhiteList.includes(String(to.name))) {
        next(
            entrance.value
                ? {
                      name: 'entrance',
                      params: { code: entrance.value },
                  }
                : {
                      name: 'login',
                  },
        );
        NProgress.done();
        return;
    }
    if (to.name === 'login' && !isLogin.value && entrance.value) {
        next({
            name: 'entrance',
            params: { code: entrance.value },
        });
        NProgress.done();
        return;
    }
    if (to.name === 'login' && isLogin.value) {
        next({
            name: 'home',
        });
        NProgress.done();
        return;
    }
    if (to.name === 'entrance' && isLogin.value) {
        if (to.params.code === entrance.value) {
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
    if (isLogin.value && isEnterprise.value && !enterpriseLicenseCheckWhiteList.includes(String(to.name))) {
        if (!isEnterpriseLicenseLoaded.value) {
            await loadProductProFromDB();
        }
        if (!isEnterpriseLicensed.value) {
            next({ name: 'EnterpriseLicenseRequired', query: { code: String(to.params.code || '') } });
            NProgress.done();
            return;
        }
    }
    if (to.name === 'EnterpriseLicenseRequired') {
        if (!isLogin.value) {
            next({
                name: 'entrance',
                params: to.params,
            });
            NProgress.done();
            return;
        }
        if (!isEnterprise.value || isEnterpriseLicensed.value) {
            next({ name: 'home' });
            NProgress.done();
            return;
        }
        return next();
    }

    if (to.path === '/apps/all' && to.query.install != undefined) {
        return next();
    }
    if (to.name === 'Expired') {
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
        const cachedRouteInfo = router.resolve(cachedRoute);
        if (cachedRouteInfo.matched.length > 0 && hasRouteAccess(cachedRouteInfo)) {
            isRedirecting = true;
            next(cachedRoute);
            NProgress.done();
            return;
        }
        localStorage.removeItem(activeMenuKey);
    }

    if (!hasRouteAccess(to)) {
        MsgError(i18n.global.t('commons.res.forbidden'));
        next(false);
        NProgress.done();
        return;
    }
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
