<template>
    <DrawerPro v-model="open" :header="drawerTitle" size="large" @close="handleClose">
        <template #content>
            <div v-loading="loading">
                <el-alert type="info" :closable="false" show-icon class="mb-4">
                    <template #title>{{ $t('toolbox.windowsService.configEditorHelper') }}</template>
                </el-alert>
                <el-tabs v-if="tabs.length > 0" v-model="activeTab">
                    <el-tab-pane v-for="tab in tabs" :key="tab.type" :name="tab.type" :label="tab.label">
                        <div class="config-path">{{ $t('toolbox.windowsService.filePath') }}: {{ tab.path }}</div>
                        <CodemirrorPro
                            v-model="contents[tab.type]"
                            :mode="resolveEditorMode(tab.path)"
                            :height-diff="280"
                            :placeholder="$t('commons.msg.noneData')"
                        />
                        <div class="mt-3">
                            <el-button type="primary" :loading="savingType === tab.type" @click="save(tab.type)">
                                {{ $t('commons.button.save') }}
                            </el-button>
                            <span class="save-helper">{{ $t('toolbox.windowsService.saveAndRestartHint') }}</span>
                        </div>
                    </el-tab-pane>
                </el-tabs>
                <el-empty v-else :description="$t('toolbox.windowsService.noConfigFiles')" />
            </div>
        </template>
    </DrawerPro>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue';
import CodemirrorPro from '@/components/codemirror-pro/index.vue';
import type { WindowsService } from '@/api/interface/windows-service';
import {
    getWindowsServiceConfigFile as getDefaultWindowsServiceConfigFile,
    updateWindowsServiceConfigFile as updateDefaultWindowsServiceConfigFile,
} from '@/api/modules/windows-service';
import {
    getWindowsServiceConfigFile as getEnhanceWindowsServiceConfigFile,
    updateWindowsServiceConfigFile as updateEnhanceWindowsServiceConfigFile,
} from '@/enhance/api';
import { MsgSuccess } from '@/utils/message';
import i18n from '@/lang';

type ConfigFileType = 'config' | 'env';

interface DrawerOptions {
    useEnhanceApi?: boolean;
    operateNode?: string;
}

const open = ref(false);
const loading = ref(false);
const savingType = ref<ConfigFileType | ''>('');
const item = ref<WindowsService.Item | null>(null);
const options = ref<DrawerOptions>({});
const activeTab = ref<ConfigFileType>('config');
const emit = defineEmits<{
    saved: [];
}>();

const contents = reactive<Record<ConfigFileType, string>>({
    config: '',
    env: '',
});

const paths = reactive<Record<ConfigFileType, string>>({
    config: '',
    env: '',
});

const tabs = computed(() => {
    const current = item.value;
    if (!current) {
        return [];
    }
    const result: Array<{ type: ConfigFileType; label: string; path: string }> = [];
    if (current.configPath) {
        result.push({
            type: 'config',
            label: i18n.global.t('toolbox.windowsService.configFileTab'),
            path: paths.config || current.configPath,
        });
    }
    if (current.envFilePath) {
        result.push({
            type: 'env',
            label: i18n.global.t('toolbox.windowsService.envFileTab'),
            path: paths.env || current.envFilePath,
        });
    }
    return result;
});

const drawerTitle = computed(() => {
    const current = item.value;
    if (!current) {
        return i18n.global.t('toolbox.windowsService.configEditorTitle');
    }
    return `${i18n.global.t('toolbox.windowsService.configEditorTitle')} - ${current.displayName || current.name}`;
});

const loadConfigFile = async (type: ConfigFileType) => {
    if (!item.value) {
        return;
    }
    const request = options.value.useEnhanceApi
        ? getEnhanceWindowsServiceConfigFile(item.value.id, type, options.value.operateNode)
        : getDefaultWindowsServiceConfigFile(item.value.id, type);
    const res = await request;
    contents[type] = res.data?.content || '';
    paths[type] = res.data?.path || '';
};

const resolveEditorMode = (path: string) => {
    const normalized = path.toLowerCase();
    if (normalized.endsWith('.yml') || normalized.endsWith('.yaml')) {
        return 'yaml';
    }
    if (normalized.endsWith('.json')) {
        return 'json';
    }
    if (normalized.endsWith('.xml')) {
        return 'xml';
    }
    return '';
};

const save = async (type: ConfigFileType) => {
    if (!item.value) {
        return;
    }
    savingType.value = type;
    try {
        if (options.value.useEnhanceApi) {
            await updateEnhanceWindowsServiceConfigFile(
                item.value.id,
                {
                    type,
                    content: contents[type],
                },
                options.value.operateNode,
            );
        } else {
            await updateDefaultWindowsServiceConfigFile(item.value.id, {
                type,
                content: contents[type],
            });
        }
        MsgSuccess(i18n.global.t('commons.msg.operationSuccess'));
        emit('saved');
    } catch {
        // http interceptor already surfaces the error message
    } finally {
        savingType.value = '';
    }
};

const acceptParams = async (currentItem: WindowsService.Item, drawerOptions?: DrawerOptions) => {
    item.value = currentItem;
    options.value = drawerOptions || {};
    contents.config = '';
    contents.env = '';
    paths.config = currentItem.configPath || '';
    paths.env = currentItem.envFilePath || '';
    activeTab.value = currentItem.configPath ? 'config' : 'env';
    open.value = true;
    if (!currentItem.configPath && !currentItem.envFilePath) {
        return;
    }
    loading.value = true;
    try {
        if (currentItem.configPath) {
            await loadConfigFile('config');
        }
        if (currentItem.envFilePath) {
            await loadConfigFile('env');
        }
    } catch {
        // http interceptor already surfaces the error message
    } finally {
        loading.value = false;
    }
};

const handleClose = () => {
    open.value = false;
    item.value = null;
    options.value = {};
    activeTab.value = 'config';
    contents.config = '';
    contents.env = '';
    paths.config = '';
    paths.env = '';
};

defineExpose({
    acceptParams,
});
</script>

<style lang="scss" scoped>
.config-path {
    margin-bottom: 12px;
    color: var(--el-text-color-secondary);
    word-break: break-all;
}

.save-helper {
    margin-left: 12px;
    color: var(--el-text-color-secondary);
    font-size: 12px;
}
</style>
