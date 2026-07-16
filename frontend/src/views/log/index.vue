<template>
    <div>
        <RouterButton :buttons="buttons" />
        <LayoutContent>
            <router-view></router-view>
        </LayoutContent>
    </div>
</template>

<script lang="ts" setup>
import { computed, onMounted } from 'vue';
import i18n from '@/lang';
import { useWindowsPanel } from '@/composables/useWindowsPanel';

const { isWindowsPanel, isOsLoaded, ensureOsInfo } = useWindowsPanel();

onMounted(() => {
    ensureOsInfo();
});

const buttons = computed(() => {
    const items = [
        {
            label: i18n.global.t('logs.panelLog'),
            path: '/logs/operation',
        },
    ];
    // SSH login logs and website logs are Linux-only; hide them on Windows.
    // Gate on isOsLoaded so the Linux-only tabs never flash on Windows before OS resolves.
    if (isOsLoaded.value && !isWindowsPanel.value) {
        items.push(
            {
                label: i18n.global.t('ssh.loginLogs'),
                path: '/logs/ssh',
            },
            {
                label: i18n.global.t('logs.websiteLog'),
                path: '/logs/website',
            },
        );
    }
    return items;
});
</script>
