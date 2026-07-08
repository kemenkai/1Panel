<template>
    <LayoutContent :title="$t('setting.enhance')" v-loading="loading">
        <template #main>
            <div class="enhance-grid">
                <div v-if="isLinuxPanel" class="enhance-card">
                    <div class="enhance-title">{{ $t('xpack.node.nodeManagement') }}</div>
                    <div class="enhance-desc">{{ $t('setting.enhanceNodeDesc') }}</div>
                    <el-button type="primary" @click="router.push({ name: 'EnhanceSimpleNode' })">
                        {{ $t('commons.button.view') }}
                    </el-button>
                </div>
                <div v-if="isWindowsPanel || isLinuxPanel" class="enhance-card">
                    <div class="enhance-title">{{ $t('setting.enhanceWindowsServiceAction') }}</div>
                    <div class="enhance-desc">{{ $t('setting.enhanceWindowsServiceDesc') }}</div>
                    <el-button type="primary" @click="router.push({ name: 'EnhanceWindowsService' })">
                        {{ $t('setting.enhanceWindowsServiceAction') }}
                    </el-button>
                </div>
                <div v-if="isLinuxPanel" class="enhance-card">
                    <div class="enhance-title">{{ $t('setting.enhanceLocalAppTitle') }}</div>
                    <div class="enhance-desc">{{ $t('setting.enhanceLocalAppDesc') }}</div>
                    <el-button type="primary" @click="router.push({ name: 'EnhanceLocalApp' })">
                        {{ $t('setting.enhanceLocalAppAction') }}
                    </el-button>
                </div>
            </div>
        </template>
    </LayoutContent>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import { loadOsInfo } from '@/api/modules/dashboard';

defineOptions({ name: 'EnhanceOverviewPage' });

const router = useRouter();
const loading = ref(false);
const os = ref('');
const platform = ref('');

const isWindowsPanel = computed(() => {
    const osValue = os.value.toLowerCase();
    const platformValue = platform.value.toLowerCase();
    return osValue.includes('windows') || platformValue.includes('windows');
});

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
    min-height: 44px;
    color: var(--el-text-color-secondary);
    line-height: 1.6;
}
</style>
