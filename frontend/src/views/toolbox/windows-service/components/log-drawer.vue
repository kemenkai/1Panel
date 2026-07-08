<template>
    <DrawerPro v-model="open" :header="drawerTitle" :size="globalStore.isFullScreen ? 'full' : 'large'" @close="handleClose">
        <template #content>
            <div v-loading="loading">
                <el-alert v-if="logInfo?.path" type="info" :closable="false" show-icon class="mb-4">
                    <template #title>{{ $t('toolbox.windowsService.logPath') }}: {{ logInfo.path }}</template>
                </el-alert>
                <LogFile
                    v-if="logInfo?.exists"
                    :config="logConfig"
                    :show-download="true"
                    :height-diff="220"
                />
                <el-empty v-else :description="$t('toolbox.windowsService.logEmpty')" />
            </div>
        </template>
    </DrawerPro>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import LogFile from '@/components/log/file/index.vue';
import { GlobalStore } from '@/store';
import type { WindowsService } from '@/api/interface/windows-service';
import { getWindowsServiceLogFile as getDefaultWindowsServiceLogFile } from '@/api/modules/windows-service';
import { getWindowsServiceLogFile as getEnhanceWindowsServiceLogFile } from '@/enhance/api';
import i18n from '@/lang';

interface DrawerOptions {
    useEnhanceApi?: boolean;
    operateNode?: string;
}

const globalStore = GlobalStore();
const open = ref(false);
const loading = ref(false);
const item = ref<WindowsService.Item | null>(null);
const logInfo = ref<WindowsService.LogFile | null>(null);
const options = ref<DrawerOptions>({});

const drawerTitle = computed(() => {
    const current = item.value;
    if (!current) {
        return i18n.global.t('toolbox.windowsService.logTitle');
    }
    return `${i18n.global.t('toolbox.windowsService.logTitle')} - ${current.displayName || current.name}`;
});

const logConfig = computed(() => {
    const current = item.value;
    return {
        id: current?.id || 0,
        type: 'windows-service',
        name: current?.name || '',
        tail: true,
        colorMode: 'system',
        operateNode: options.value.operateNode || '',
    };
});

const acceptParams = async (currentItem: WindowsService.Item, drawerOptions?: DrawerOptions) => {
    item.value = currentItem;
    options.value = drawerOptions || {};
    open.value = true;
    loading.value = true;
    try {
        const request = options.value.useEnhanceApi
            ? getEnhanceWindowsServiceLogFile(currentItem.id, options.value.operateNode)
            : getDefaultWindowsServiceLogFile(currentItem.id);
        const res = await request;
        logInfo.value = res.data || null;
    } finally {
        loading.value = false;
    }
};

const handleClose = () => {
    open.value = false;
    item.value = null;
    logInfo.value = null;
    options.value = {};
};

defineExpose({
    acceptParams,
});
</script>
