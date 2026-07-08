<template>
    <LayoutContent :title="$t('xpack.node.nodeManagement')" v-loading="loading">
        <template #leftToolBar>
            <el-button @click="router.push({ name: 'EnhanceHome' })">{{ $t('commons.button.back') }}</el-button>
            <el-button type="primary" :disabled="refreshingAll" @click="openCreate">{{ $t('commons.button.create') }}</el-button>
        </template>
        <template #rightToolBar>
            <el-button link type="primary" :loading="refreshingAll" @click="refreshAll">
                {{ $t('commons.button.refresh') }}
            </el-button>
        </template>
        <template #main>
            <el-alert
                class="mb-4"
                type="info"
                :closable="false"
                show-icon
                :title="$t('xpack.node.communityHelper')"
            />
            <el-table :data="items">
                <el-table-column prop="name" :label="$t('commons.table.name')" min-width="160" />
                <el-table-column prop="addr" :label="$t('setting.address')" min-width="220" />
                <el-table-column prop="securityEntrance" :label="$t('setting.entrance')" min-width="180">
                    <template #default="{ row }">
                        {{ row.securityEntrance || '-' }}
                    </template>
                </el-table-column>
                <el-table-column prop="description" :label="$t('commons.table.description')" min-width="200">
                    <template #default="{ row }">
                        {{ row.description || '-' }}
                    </template>
                </el-table-column>
                <el-table-column prop="lastCheckAt" :label="$t('commons.table.updatedAt')" min-width="180">
                    <template #default="{ row }">
                        {{ row.lastCheckAt ? dateFormat(null, null, row.lastCheckAt) : '-' }}
                    </template>
                </el-table-column>
                <el-table-column prop="status" :label="$t('commons.table.status')" width="120">
                    <template #default="{ row }">
                        <Status :status="row.status" :msg="row.message" />
                    </template>
                </el-table-column>
                <el-table-column :label="$t('commons.table.operate')" width="240" fixed="right">
                    <template #default="{ row }">
                        <el-button link type="primary" :disabled="isRowBusy(row.id)" @click="visit(row)">
                            {{ $t('commons.button.visit') }}
                        </el-button>
                        <el-button link type="primary" :loading="isRowBusy(row.id)" @click="refreshRow(row)">
                            {{ $t('commons.button.refresh') }}
                        </el-button>
                        <el-button link type="primary" :disabled="isRowBusy(row.id)" @click="openEdit(row)">
                            {{ $t('commons.button.edit') }}
                        </el-button>
                        <el-button link type="danger" :disabled="isRowBusy(row.id)" @click="remove(row)">
                            {{ $t('commons.button.delete') }}
                        </el-button>
                    </template>
                </el-table-column>
            </el-table>
        </template>
    </LayoutContent>

    <DrawerPro v-model="drawerOpen" :header="form.id ? $t('commons.button.edit') : $t('commons.button.create')" size="small">
        <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
            <el-form-item :label="$t('commons.table.name')" prop="name">
                <el-input v-model.trim="form.name" />
            </el-form-item>
            <el-form-item :label="$t('setting.address')" prop="addr">
                <el-input v-model.trim="form.addr" placeholder="https://panel.example.com:9999" />
            </el-form-item>
            <el-form-item :label="$t('setting.entrance')" prop="securityEntrance">
                <el-input v-model.trim="form.securityEntrance" placeholder="entrance-path" />
            </el-form-item>
            <el-form-item :label="$t('setting.apiKey')" prop="apiKey">
                <el-input v-model.trim="form.apiKey" type="password" show-password />
            </el-form-item>
            <el-form-item :label="$t('commons.table.description')" prop="description">
                <el-input v-model.trim="form.description" type="textarea" :rows="3" />
            </el-form-item>
        </el-form>
        <template #footer>
            <el-button @click="drawerOpen = false">{{ $t('commons.button.cancel') }}</el-button>
            <el-button :loading="checking" @click="verifyForm(formRef)">{{ $t('commons.button.verify') }}</el-button>
            <el-button type="primary" :loading="submitting" @click="submit(formRef)">
                {{ $t('commons.button.confirm') }}
            </el-button>
        </template>
    </DrawerPro>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';
import type { FormInstance, FormRules } from 'element-plus';
import { useRouter } from 'vue-router';
import {
    checkSimpleNode,
    createSimpleNode,
    deleteSimpleNode,
    listAllSimpleNodes,
    refreshAllSimpleNodes,
    refreshSimpleNode,
    updateSimpleNode,
} from '@/enhance/api';
import { MsgSuccess, MsgWarning } from '@/utils/message';
import { Rules } from '@/global/form-rules';
import type { Enhance } from '@/enhance/types';
import Status from '@/components/status/index.vue';
import i18n from '@/lang';
import { dateFormat } from '@/utils/date';
import { openSimpleNodePage } from '@/enhance/simple-node';

defineOptions({ name: 'EnhanceSimpleNodePage' });

const loading = ref(false);
const submitting = ref(false);
const checking = ref(false);
const refreshingAll = ref(false);
const refreshingRowIds = ref<number[]>([]);
const items = ref<Enhance.SimpleNodeItem[]>([]);
const drawerOpen = ref(false);
const formRef = ref<FormInstance>();
const router = useRouter();

const form = reactive<Enhance.SimpleNodeUpdate>({
    id: 0,
    name: '',
    addr: '',
    securityEntrance: '',
    apiKey: '',
    description: '',
});

const rules = reactive<FormRules>({
    name: [Rules.requiredInput],
    addr: [Rules.requiredInput, Rules.paramHttp],
});

const resetForm = () => {
    form.id = 0;
    form.name = '';
    form.addr = '';
    form.securityEntrance = '';
    form.apiKey = '';
    form.description = '';
};

const loadData = async () => {
    loading.value = true;
    try {
        const res = await listAllSimpleNodes();
        items.value = res.data || [];
    } finally {
        loading.value = false;
    }
};

const openCreate = () => {
    resetForm();
    drawerOpen.value = true;
};

const openEdit = (row: Enhance.SimpleNodeItem) => {
    form.id = row.id;
    form.name = row.name;
    form.addr = row.addr;
    form.securityEntrance = row.securityEntrance || '';
    form.apiKey = row.apiKey || '';
    form.description = row.description || '';
    drawerOpen.value = true;
};

const submit = async (formEl?: FormInstance) => {
    if (!formEl) return;
    await formEl.validate(async (valid) => {
        if (!valid) return;
        submitting.value = true;
        try {
            if (form.id) {
                await updateSimpleNode({ ...form });
            } else {
                await createSimpleNode({
                    name: form.name,
                    addr: form.addr,
                    securityEntrance: form.securityEntrance,
                    apiKey: form.apiKey,
                    description: form.description,
                });
            }
            drawerOpen.value = false;
            MsgSuccess(i18n.global.t('commons.msg.operationSuccess'));
            await loadData();
        } finally {
            submitting.value = false;
        }
    });
};

const syncCheckedItem = (item: Enhance.SimpleNodeItem) => {
    const index = items.value.findIndex((current) => current.id === item.id);
    if (index !== -1) {
        items.value[index] = { ...items.value[index], ...item };
    }
};

const updatePendingRow = (id: number, message: string) => {
    const index = items.value.findIndex((current) => current.id === id);
    if (index !== -1) {
        items.value[index] = {
            ...items.value[index],
            status: 'Waiting',
            message,
        };
    }
};

const isRowBusy = (id: number) => {
    return refreshingAll.value || refreshingRowIds.value.includes(id);
};

const setRowRefreshing = (id: number, value: boolean) => {
    if (value) {
        if (!refreshingRowIds.value.includes(id)) {
            refreshingRowIds.value = [...refreshingRowIds.value, id];
        }
        return;
    }
    refreshingRowIds.value = refreshingRowIds.value.filter((item) => item !== id);
};

const verifyResult = (item: Enhance.SimpleNodeItem) => {
    if (item.status === 'Healthy') {
        MsgSuccess(i18n.global.t('commons.msg.operationSuccess'));
        return;
    }
    MsgWarning(item.message || i18n.global.t('xpack.node.nodeUnhealthy'));
};

const formatFailureReason = (message?: string) => {
    const normalized = (message || i18n.global.t('xpack.node.nodeUnhealthy')).replace(/\s+/g, ' ').trim();
    if (normalized.length <= 48) {
        return normalized;
    }
    return `${normalized.slice(0, 48)}...`;
};

const refreshRow = async (row: Enhance.SimpleNodeItem) => {
    const snapshot = { ...row };
    setRowRefreshing(row.id, true);
    updatePendingRow(row.id, i18n.global.t('xpack.node.refreshing'));
    try {
        const res = await refreshSimpleNode(row.id);
        syncCheckedItem(res.data);
        verifyResult(res.data);
    } catch (error) {
        syncCheckedItem(snapshot);
    } finally {
        setRowRefreshing(row.id, false);
    }
};

const verifyForm = async (formEl?: FormInstance) => {
    if (!formEl) return;
    await formEl.validate(async (valid) => {
        if (!valid) return;
        checking.value = true;
        try {
            const res = await checkSimpleNode({
                id: form.id || undefined,
                name: form.name,
                addr: form.addr,
                securityEntrance: form.securityEntrance,
                apiKey: form.apiKey,
            });
            verifyResult(res.data);
        } finally {
            checking.value = false;
        }
    });
};

const refreshAll = async () => {
    const snapshot = items.value.map((item) => ({ ...item }));
    refreshingAll.value = true;
    refreshingRowIds.value = [];
    try {
        items.value = items.value.map((item) => ({
            ...item,
            status: 'Waiting',
            message: i18n.global.t('xpack.node.refreshing'),
        }));
        const res = await refreshAllSimpleNodes();
        items.value = res.data || [];
        const unhealthyItems = items.value.filter((item) => item.status !== 'Healthy');
        const summary = i18n.global.t('xpack.node.refreshSummary', [
            items.value.length,
            items.value.length - unhealthyItems.length,
            unhealthyItems.length,
        ]);
        if (unhealthyItems.length === 0) {
            MsgSuccess(summary);
            return;
        }
        const failedNames = unhealthyItems
            .slice(0, 5)
            .map((item) => item.name)
            .join(', ');
        const detail = i18n.global.t('xpack.node.refreshFailedNodes', [failedNames]);
        const reasonDetails = unhealthyItems
            .slice(0, 3)
            .map((item) => `${item.name}(${formatFailureReason(item.message)})`)
            .join('; ');
        const reason = i18n.global.t('xpack.node.refreshFailedReasons', [reasonDetails]);
        MsgWarning(`${summary} ${detail} ${reason}`);
    } catch (error) {
        items.value = snapshot;
    } finally {
        refreshingAll.value = false;
    }
};

const visit = async (row: Enhance.SimpleNodeItem) => {
    await openSimpleNodePage(row.id);
};

const remove = async (row: Enhance.SimpleNodeItem) => {
    await ElMessageBox.confirm(row.name, i18n.global.t('commons.button.delete'), {
        type: 'warning',
    });
    await deleteSimpleNode(row.id);
    MsgSuccess(i18n.global.t('commons.msg.operationSuccess'));
    await loadData();
};

onMounted(() => {
    loadData();
});
</script>
