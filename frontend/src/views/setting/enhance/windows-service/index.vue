<template>
    <LayoutContent :title="$t('setting.enhanceWindowsServiceAction')" v-loading="loading">
        <template #prompt>
            <el-alert type="info" :closable="false" show-icon class="mb-3">
                <template #title>
                    {{ $t('toolbox.windowsService.helper') }}
                </template>
            </el-alert>
            <el-alert type="success" :closable="false" show-icon class="mb-3">
                <template #title>
                    {{ $t('setting.enhanceWindowsServiceHelper') }}
                </template>
            </el-alert>
            <el-alert v-if="!isSupportedPanel" type="warning" :closable="false" show-icon>
                <template #title>
                    {{ $t('setting.enhanceWindowsServiceNodeTip') }}
                </template>
            </el-alert>
        </template>
        <template #leftToolBar>
            <el-button @click="router.push({ name: 'EnhanceHome' })">{{ $t('commons.button.back') }}</el-button>
            <el-button type="primary" :disabled="!isSupportedPanel" @click="openCreate">{{ $t('commons.button.create') }}</el-button>
        </template>
        <template #rightToolBar>
            <el-button link type="primary" @click="loadData">{{ $t('commons.button.refresh') }}</el-button>
        </template>
        <template #main>
            <el-table :data="items">
                <el-table-column prop="name" :label="$t('commons.table.name')" min-width="140" />
                <el-table-column prop="displayName" :label="$t('commons.table.title')" min-width="140" />
                <el-table-column prop="serviceType" :label="$t('commons.table.type')" min-width="100" />
                <el-table-column :label="$t('toolbox.windowsService.registrationStatus')" width="120">
                    <template #default="{ row }">
                        <el-tag size="small" :type="row.registerService ? 'success' : 'warning'" round effect="light">
                            {{
                                row.registerService
                                    ? $t('toolbox.windowsService.registrationEnabled')
                                    : $t('toolbox.windowsService.registrationDisabled')
                            }}
                        </el-tag>
                    </template>
                </el-table-column>
                <el-table-column
                    prop="servicePath"
                    :label="$t('toolbox.windowsService.servicePath')"
                    min-width="220"
                    show-overflow-tooltip
                />
                <el-table-column
                    prop="uninstallScriptPath"
                    :label="$t('toolbox.windowsService.uninstallScriptPath')"
                    min-width="220"
                    show-overflow-tooltip
                />
                <el-table-column prop="status" :label="$t('commons.table.status')" width="120">
                    <template #default="{ row }">
                        <Status :status="row.status" :msg="row.message" />
                    </template>
                </el-table-column>
                <el-table-column :label="$t('commons.table.operate')" width="360" fixed="right">
                    <template #default="{ row }">
                        <el-button
                            link
                            type="primary"
                            :disabled="!isSupportedPanel || !row.registerService"
                            @click="operate(row.id, row.status === 'Healthy' ? 'stop' : 'start')"
                        >
                            {{ row.status === 'Healthy' ? $t('commons.button.stop') : $t('commons.button.start') }}
                        </el-button>
                        <el-button
                            link
                            type="primary"
                            :disabled="!isSupportedPanel || !row.registerService"
                            @click="operate(row.id, 'restart')"
                        >
                            {{ $t('commons.button.restart') }}
                        </el-button>
                        <el-button link type="primary" :disabled="!isSupportedPanel" @click="openEdit(row)">
                            {{ $t('commons.button.edit') }}
                        </el-button>
                        <el-button link type="primary" @click="openConfigEditor(row)">
                            {{ $t('toolbox.windowsService.editConfig') }}
                        </el-button>
                        <el-button link type="primary" @click="openLogDrawer(row)">
                            {{ $t('toolbox.windowsService.viewLog') }}
                        </el-button>
                        <el-button link type="danger" :disabled="!isSupportedPanel" @click="remove(row.id)">
                            {{ $t('commons.button.delete') }}
                        </el-button>
                    </template>
                </el-table-column>
            </el-table>
        </template>
    </LayoutContent>
    <Create ref="createRef" @close="loadData" />
    <ConfigEditorDrawer ref="configDrawerRef" @saved="loadData" />
    <LogDrawer ref="logDrawerRef" />
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import Status from '@/components/status/index.vue';
import Create from '@/views/toolbox/windows-service/create/index.vue';
import ConfigEditorDrawer from '@/views/toolbox/windows-service/components/config-editor-drawer.vue';
import LogDrawer from '@/views/toolbox/windows-service/components/log-drawer.vue';
import { loadOsInfo } from '@/api/modules/dashboard';
import { deleteWindowsService, listWindowsServices, operateWindowsService } from '@/enhance/api';
import type { WindowsService } from '@/api/interface/windows-service';
import { MsgSuccess } from '@/utils/message';
import i18n from '@/lang';

defineOptions({ name: 'EnhanceWindowsServicePage' });

const loading = ref(false);
const items = ref<WindowsService.Item[]>([]);
const createRef = ref();
const configDrawerRef = ref();
const logDrawerRef = ref();
const os = ref('');
const platform = ref('');
const router = useRouter();

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

const isSupportedPanel = computed(() => isWindowsPanel.value || isLinuxPanel.value);

const loadSystemInfo = async () => {
    const res = await loadOsInfo();
    os.value = res.data.os || '';
    platform.value = res.data.platform || '';
};

const loadData = async () => {
    loading.value = true;
    try {
        await loadSystemInfo();
        const res = await listWindowsServices();
        items.value = res.data || [];
    } finally {
        loading.value = false;
    }
};

const openCreate = () => {
    createRef.value?.acceptParams();
};

const openEdit = async (item: WindowsService.Item) => {
    await loadData();
    const latestItem = items.value.find((service) => service.id === item.id) ?? item;
    createRef.value?.acceptParams(latestItem);
};

const openConfigEditor = (item: WindowsService.Item) => {
    configDrawerRef.value?.acceptParams(item, { useEnhanceApi: true });
};

const openLogDrawer = (item: WindowsService.Item) => {
    logDrawerRef.value?.acceptParams(item, { useEnhanceApi: true });
};

const operate = async (id: number, op: WindowsService.Operate['operate']) => {
    await operateWindowsService({ id, operate: op });
    MsgSuccess(i18n.global.t('commons.msg.operationSuccess'));
    await loadData();
};

const remove = async (id: number) => {
    await ElMessageBox.confirm(i18n.global.t('commons.msg.delete'), i18n.global.t('commons.button.delete'), {
        type: 'warning',
    });
    await deleteWindowsService(id);
    MsgSuccess(i18n.global.t('commons.msg.operationSuccess'));
    await loadData();
};

onMounted(() => {
    loadData();
});
</script>
