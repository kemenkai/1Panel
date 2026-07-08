<template>
    <LayoutContent :title="$t('setting.enhanceLocalAppTitle')" v-loading="loading">
        <template #prompt>
            <el-alert type="info" :closable="false" show-icon class="mb-3">
                <template #title>
                    {{ $t('setting.enhanceLocalAppHelper') }}
                </template>
            </el-alert>
            <el-alert v-if="!isLinuxPanel" type="warning" :closable="false" show-icon>
                <template #title>
                    {{ $t('setting.enhanceLocalAppOnlyLinux') }}
                </template>
            </el-alert>
        </template>
        <template #leftToolBar>
            <el-button @click="router.push({ name: 'EnhanceHome' })">{{ $t('commons.button.back') }}</el-button>
            <el-button type="primary" :disabled="!isLinuxPanel" :loading="syncing" @click="syncLocal">
                {{ $t('app.syncLocalApp') }}
            </el-button>
            <el-button :disabled="!isLinuxPanel" @click="openUploadLocalPackage">
                {{ $t('app.uploadLocalAppPackage') }}
            </el-button>
        </template>
        <template #rightToolBar>
            <el-button link type="primary" @click="router.push({ name: 'AppAll' })">
                {{ $t('setting.enhanceLocalAppAction') }}
            </el-button>
            <el-button link type="primary" @click="router.push({ name: 'AppInstalled' })">
                {{ $t('app.installed') }}
            </el-button>
        </template>
        <template #main>
            <div class="enhance-grid">
                <div class="enhance-card">
                    <div class="enhance-title">{{ $t('app.syncLocalApp') }}</div>
                    <div class="enhance-desc">{{ $t('setting.enhanceLocalAppSyncDesc') }}</div>
                    <el-button type="primary" :disabled="!isLinuxPanel" :loading="syncing" @click="syncLocal">
                        {{ $t('app.syncLocalApp') }}
                    </el-button>
                </div>
                <div class="enhance-card">
                    <div class="enhance-title">{{ $t('app.uploadLocalAppPackage') }}</div>
                    <div class="enhance-desc">{{ $t('setting.enhanceLocalAppUploadDesc') }}</div>
                    <el-button type="primary" :disabled="!isLinuxPanel" @click="openUploadLocalPackage">
                        {{ $t('app.uploadLocalAppPackage') }}
                    </el-button>
                </div>
                <div class="enhance-card">
                    <div class="enhance-title">{{ $t('menu.apps') }}</div>
                    <div class="enhance-desc">{{ $t('setting.enhanceLocalAppBrowseDesc') }}</div>
                    <div class="enhance-actions">
                        <el-button type="primary" @click="router.push({ name: 'AppAll' })">
                            {{ $t('setting.enhanceLocalAppAction') }}
                        </el-button>
                        <el-button @click="router.push({ name: 'AppInstalled' })">{{ $t('app.installed') }}</el-button>
                    </div>
                </div>
            </div>
        </template>
    </LayoutContent>

    <TaskLog ref="taskLogRef" />
    <UploadLocalPackage ref="uploadLocalPackageRef" @uploaded="handleLocalPackageUploaded" />
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import { loadOsInfo } from '@/api/modules/dashboard';
import { syncLocalApp } from '@/api/modules/app';
import TaskLog from '@/components/log/task/index.vue';
import UploadLocalPackage from '@/views/app-store/apps/upload-local-package.vue';
import { newUUID } from '@/utils/id';

defineOptions({ name: 'EnhanceLocalAppPage' });

const router = useRouter();
const loading = ref(false);
const syncing = ref(false);
const os = ref('');
const platform = ref('');
const taskLogRef = ref();
const uploadLocalPackageRef = ref();

const isLinuxPanel = computed(() => {
    const osValue = os.value.toLowerCase();
    const platformValue = platform.value.toLowerCase();
    return osValue.includes('linux') || platformValue.includes('linux');
});

const loadSystemInfo = async () => {
    loading.value = true;
    try {
        const res = await loadOsInfo();
        os.value = res.data.os || '';
        platform.value = res.data.platform || '';
    } finally {
        loading.value = false;
    }
};

const openTaskLog = (taskID: string) => {
    taskLogRef.value?.openWithTaskID(taskID);
};

const syncLocal = async () => {
    if (!isLinuxPanel.value) {
        return;
    }
    syncing.value = true;
    const taskID = newUUID();
    try {
        await syncLocalApp({ taskID });
        openTaskLog(taskID);
    } finally {
        syncing.value = false;
    }
};

const openUploadLocalPackage = () => {
    uploadLocalPackageRef.value?.acceptParams();
};

const handleLocalPackageUploaded = (payload: { taskID: string }) => {
    if (payload.taskID) {
        openTaskLog(payload.taskID);
    }
};

onMounted(() => {
    loadSystemInfo();
});
</script>

<style scoped lang="scss">
.enhance-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
    gap: 16px;
}

.enhance-card {
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 20px;
    border: 1px solid var(--el-border-color);
    border-radius: 12px;
    background: var(--el-bg-color);
}

.enhance-title {
    font-size: 16px;
    font-weight: 600;
    color: var(--el-text-color-primary);
}

.enhance-desc {
    min-height: 66px;
    color: var(--el-text-color-secondary);
    line-height: 1.6;
}

.enhance-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 12px;
}
</style>
