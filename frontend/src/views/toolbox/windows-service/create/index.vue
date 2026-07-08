<template>
    <DrawerPro
        v-model="open"
        :header="form.id ? $t('commons.button.edit') : $t('commons.button.create')"
        size="small"
        :auto-close="false"
        :back="handleDrawerSoftClose"
        @close="handleDrawerSoftClose"
    >
        <el-form ref="formRef" label-position="top" :model="form" :rules="rules">
            <el-form-item :label="$t('toolbox.windowsService.template')">
                <el-select v-model="selectedTemplate" class="w-full" @change="applyTemplate">
                    <el-option :label="$t('toolbox.windowsService.javaTemplate')" value="java" />
                    <el-option v-if="!isLinux" :label="$t('toolbox.windowsService.dllTemplate')" value="dll" />
                    <el-option :label="$t('toolbox.windowsService.packageTemplate')" value="package" />
                </el-select>
                <span class="input-help">{{ $t('toolbox.windowsService.templateHelper') }}</span>
            </el-form-item>
            <el-form-item v-if="!isPackage" :label="$t('commons.table.name')" prop="name">
                <el-input v-model.trim="form.name" :disabled="!!form.id" />
                <span class="input-help">{{ $t('toolbox.windowsService.nameHelper') }}</span>
            </el-form-item>
            <el-form-item v-if="!isPackage" :label="$t('commons.table.title')" prop="displayName">
                <el-input v-model.trim="form.displayName" />
                <span class="input-help">{{ $t('toolbox.windowsService.displayNameHelper') }}</span>
            </el-form-item>
            <el-form-item v-if="!isPackage" :label="$t('toolbox.windowsService.serviceType')" prop="serviceType">
                <el-select v-model="form.serviceType" class="w-full" @change="onServiceTypeChange">
                    <el-option label="Java" value="java" />
                    <el-option v-if="!isLinux" label="DLL" value="dll" />
                    <el-option label="Package" value="package" />
                </el-select>
                <span class="input-help">{{ $t('toolbox.windowsService.serviceTypeHelper') }}</span>
            </el-form-item>

            <template v-if="isJavaLike">
                <el-form-item :label="$t('toolbox.windowsService.jarPath')" prop="jarPath">
                    <el-upload
                        ref="uploadRef"
                        v-model:file-list="jarFileList"
                        drag
                        :auto-upload="false"
                        :limit="1"
                        :accept="uploadAccept"
                        :on-exceed="handleExceed"
                        :before-upload="beforeJarUpload"
                        :on-change="handleJarChange"
                    >
                        <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
                        <div class="el-upload__text">
                            {{ $t('file.dropHelper') }}
                            <em>{{ $t('file.clickHelper') }}</em>
                        </div>
                    </el-upload>
                    <div v-if="currentJarPath" class="input-help">{{ $t('toolbox.windowsService.jarCurrentPath', [currentJarPath]) }}</div>
                    <div v-if="jarTargetPath" class="input-help">{{ $t('toolbox.windowsService.jarTargetPath', [jarTargetPath]) }}</div>
                    <span class="input-help">{{ jarUploadHelper }}</span>
                </el-form-item>
                <el-form-item
                    v-if="form.serviceType === 'package'"
                    :label="$t('toolbox.windowsService.packageTemplateDir')"
                    prop="workDir"
                >
                    <el-input
                        v-model.trim="packageTemplateDir"
                        @input="handlePackageTemplateDirInput"
                        @change="handlePackageTemplateDirChange"
                    />
                    <span class="input-help">{{ $t('toolbox.windowsService.packageTemplateDirHelper') }}</span>
                </el-form-item>
                <el-form-item v-else :label="$t('toolbox.windowsService.workDir')">
                    <el-input :model-value="managedWorkDir" disabled />
                    <span class="input-help">{{ $t('toolbox.windowsService.workDirAutoHelper') }}</span>
                </el-form-item>

                <template v-if="isPackage">
                    <el-divider content-position="left">{{ $t('toolbox.windowsService.packagePreviewTitle') }}</el-divider>
                    <el-table v-if="packageServicePreview.length > 0" v-loading="previewLoading" :data="packageServicePreview">
                        <el-table-column prop="name" :label="$t('toolbox.windowsService.previewName')" min-width="140" />
                        <el-table-column prop="displayName" :label="$t('toolbox.windowsService.previewDisplayName')" min-width="160" />
                        <el-table-column :label="$t('toolbox.windowsService.previewRuntime')" width="150">
                            <template #default="{ row }">
                                <el-tag>{{ formatRuntimeType(row) }}</el-tag>
                            </template>
                        </el-table-column>
                        <el-table-column :label="$t('toolbox.windowsService.previewSourcePath')" min-width="180">
                            <template #default="{ row }">
                                {{ row.sourcePath || row.jarPath || row.workDir || '-' }}
                            </template>
                        </el-table-column>
                    </el-table>
                    <el-alert
                        v-else
                        :title="$t('toolbox.windowsService.packagePreviewEmpty')"
                        type="info"
                        :closable="false"
                        show-icon
                    />
                </template>

                <el-divider content-position="left">{{ $t('toolbox.windowsService.configTemplateTitle') }}</el-divider>
                <el-form-item :label="$t('toolbox.windowsService.configTemplateSwitch')">
                    <el-switch v-model="configTemplateEnabled" />
                    <span class="input-help">{{ configTemplateSwitchHelper }}</span>
                </el-form-item>

                <template v-if="form.configTemplate?.enabled">
                    <el-form-item :label="$t('toolbox.windowsService.configPath')">
                        <el-input :model-value="managedConfigPath" disabled />
                        <span class="input-help">{{ $t('toolbox.windowsService.configPathAutoHelper') }}</span>
                    </el-form-item>
                    <el-form-item v-if="dynamicConfigFields.length > 0" :label="$t('toolbox.windowsService.dynamicFieldTitle')">
                        <span class="input-help">{{ $t('toolbox.windowsService.configTemplateHelper') }}</span>
                    </el-form-item>
                    <el-row v-if="dynamicConfigFields.length > 0" :gutter="12">
                        <el-col v-for="field in dynamicConfigFields" :key="field.envKey" :span="getConfigFieldSpan(field)">
                            <el-form-item :label="field.envKey">
                                <el-input v-if="field.inputType === 'password'" v-model="field.value" show-password />
                                <el-input v-else v-model="field.value" :type="field.inputType === 'number' ? 'number' : 'text'" />
                                <span class="input-help">{{ buildDynamicFieldHelper(field) }}</span>
                            </el-form-item>
                        </el-col>
                    </el-row>
                    <el-alert
                        v-else
                        :title="$t('toolbox.windowsService.dynamicFieldEmpty')"
                        type="info"
                        :closable="false"
                        show-icon
                    />
                </template>

                <el-form-item v-else :label="$t('toolbox.windowsService.configPath')" prop="configPath">
                    <el-input v-model.trim="form.configPath" />
                    <span class="input-help">{{ configPathHelper }}</span>
                </el-form-item>
            </template>

            <template v-else>
                <el-form-item :label="$t('toolbox.windowsService.execPath')" prop="execPath">
                    <el-input v-model.trim="form.execPath" />
                    <span class="input-help">{{ execPathHelper }}</span>
                </el-form-item>
                <el-form-item :label="$t('toolbox.windowsService.workDir')" prop="workDir">
                    <el-input v-model.trim="form.workDir" />
                    <span class="input-help">{{ workDirHelper }}</span>
                </el-form-item>
                <el-form-item :label="$t('toolbox.windowsService.configPath')" prop="configPath">
                    <el-input v-model.trim="form.configPath" />
                    <span class="input-help">{{ configPathHelper }}</span>
                </el-form-item>
            </template>

            <el-form-item v-if="!isPackage" :label="$t('toolbox.windowsService.args')" prop="args">
                <el-input v-model="form.args" type="textarea" :rows="3" />
                <span class="input-help">{{ argsHelper }}</span>
            </el-form-item>
            <el-form-item v-if="!isPackage && !isLinux" :label="$t('toolbox.windowsService.dllDir')" prop="dllDir">
                <el-input v-model.trim="form.dllDir" />
                <span class="input-help">{{ dllDirHelper }}</span>
            </el-form-item>
            <el-form-item v-if="!isPackage" :label="$t('toolbox.windowsService.envFilePath')" prop="envFilePath">
                <el-input v-model.trim="form.envFilePath" />
                <span class="input-help">{{ $t('toolbox.windowsService.envFilePathHelper') }}</span>
            </el-form-item>

            <template v-if="isJava && !isLinux">
                <el-form-item :label="$t('toolbox.windowsService.winswPath')">
                    <el-input :model-value="managedWinSWPath" disabled />
                    <span class="input-help">{{ $t('toolbox.windowsService.winswPathAutoHelper') }}</span>
                </el-form-item>
            </template>
            <el-form-item v-else-if="!isPackage && !isLinux" :label="$t('toolbox.windowsService.winswPath')" prop="winswPath">
                <el-input v-model.trim="form.winswPath" />
                <span class="input-help">{{ winswPathHelper }}</span>
            </el-form-item>

            <el-form-item :label="$t('toolbox.windowsService.registerService')" prop="registerService">
                <el-switch v-model="form.registerService" />
                <span class="input-help">{{ $t('toolbox.windowsService.registerServiceHelper') }}</span>
            </el-form-item>
            <el-form-item :label="$t('toolbox.windowsService.autoStart')" prop="autoStart">
                <el-switch v-model="form.autoStart" />
                <span class="input-help">{{ autoStartHelper }}</span>
            </el-form-item>
        </el-form>
        <template #footer>
            <el-button @click="handleClose">{{ $t('commons.button.cancel') }}</el-button>
            <el-button type="primary" :loading="loading" @click="submit(formRef)">
                {{ confirmButtonText }}
            </el-button>
        </template>
    </DrawerPro>
</template>

<script setup lang="ts">
import type { FormInstance, FormRules, UploadInstance, UploadProps, UploadRawFile, UploadUserFile } from 'element-plus';
import { genFileId } from 'element-plus';
import { UploadFilled } from '@element-plus/icons-vue';
import { computed, nextTick, ref } from 'vue';
import type { WindowsService } from '@/api/interface/windows-service';
import {
    createWindowsService,
    getWindowsServiceConfigTemplate,
    updateWindowsService,
    uploadWindowsServicePackage,
} from '@/api/modules/windows-service';
import { loadBaseDir } from '@/api/modules/setting';
import { loadOsInfo } from '@/api/modules/dashboard';
import { MsgError, MsgInfo, MsgSuccess } from '@/utils/message';
import i18n from '@/lang';
import { Rules } from '@/global/form-rules';

type WindowsServiceTemplateType = 'java' | 'dll' | 'package';
type WindowsServiceConfigTemplate = WindowsService.ConfigTemplate;
type WindowsServiceConfigTemplateField = WindowsService.ConfigTemplateField;
type WindowsServicePreviewItem = WindowsService.UploadServicePreview;

const open = ref(false);
const loading = ref(false);
const formRef = ref<FormInstance>();
const uploadRef = ref<UploadInstance>();
const selectedTemplate = ref<WindowsServiceTemplateType>('java');
const os = ref('');
const platformName = ref('');
const installDir = ref('C:\\1Panel');

const isLinux = computed(() => {
    const osValue = os.value.toLowerCase();
    const platformValue = platformName.value.toLowerCase();
    return osValue.includes('linux') || platformValue.includes('linux');
});
const jarFileList = ref<UploadUserFile[]>([]);
const packageTemplateDir = ref('');
const draftContextKey = ref('');
const uploadedPackageFingerprint = ref('');
const packageServicePreview = ref<WindowsServicePreviewItem[]>([]);
const previewLoading = ref(false);

const buildEmptyConfigTemplate = (enabled = true): WindowsServiceConfigTemplate => ({
    enabled,
    template: 'spring-boot-application-dev',
    fileName: 'application-dev.yml',
    generatedPath: '',
    content: '',
    dynamicFields: [],
});

const cloneConfigTemplateField = (field: WindowsServiceConfigTemplateField): WindowsServiceConfigTemplateField => ({
    envKey: field.envKey,
    defaultValue: field.defaultValue,
    value: field.value,
    configPath: field.configPath,
    inputType: field.inputType,
    sensitive: field.sensitive,
});

const cloneConfigTemplate = (template?: WindowsServiceConfigTemplate | null, enabled = true): WindowsServiceConfigTemplate => {
    const baseTemplate = template ?? buildEmptyConfigTemplate(enabled);
    return {
        enabled,
        template: baseTemplate.template || 'spring-boot-application-dev',
        fileName: baseTemplate.fileName || 'application-dev.yml',
        generatedPath: baseTemplate.generatedPath || '',
        content: baseTemplate.content || '',
        dynamicFields: (baseTemplate.dynamicFields || []).map(cloneConfigTemplateField),
    };
};

const buildFieldValueMap = (fields: WindowsServiceConfigTemplateField[] = []) => {
    return fields.reduce<Record<string, WindowsServiceConfigTemplateField>>((result, field) => {
        if (field.envKey) {
            result[field.envKey] = field;
        }
        return result;
    }, {});
};

const mergeConfigTemplate = (
    baseTemplate?: WindowsServiceConfigTemplate | null,
    currentTemplate?: WindowsServiceConfigTemplate | null,
    enabled = true,
): WindowsServiceConfigTemplate => {
    const base = cloneConfigTemplate(baseTemplate, enabled);
    const currentFieldMap = buildFieldValueMap(currentTemplate?.dynamicFields || []);
    base.dynamicFields = base.dynamicFields.map((field) => {
        const currentField = currentFieldMap[field.envKey];
        if (!currentField) {
            return field;
        }
        return {
            ...field,
            value: currentField.value,
        };
    });
    if ((!base.template || !base.fileName) && currentTemplate) {
        base.template = currentTemplate.template || base.template;
        base.fileName = currentTemplate.fileName || base.fileName;
    }
    if (!base.content && currentTemplate?.content) {
        base.content = currentTemplate.content;
    }
    return base;
};

const initForm = (): WindowsService.Update => ({
    id: 0,
    name: '',
    displayName: '',
    serviceType: 'java',
    execPath: '',
    args: '',
    workDir: '',
    envFilePath: '',
    dllDir: '',
    configPath: '',
    jarPath: '',
    winswPath: '',
    configTemplate: buildEmptyConfigTemplate(true),
    registerService: true,
    autoStart: true,
});

const form = ref<WindowsService.Update>(initForm());

const templateMap: Record<WindowsServiceTemplateType, Partial<WindowsService.Update>> = {
    java: {
        serviceType: 'java',
        displayName: 'Java Service',
        execPath: '',
        args: '-Xms512m -Xmx512m',
        workDir: '',
        jarPath: '',
        dllDir: '',
        configPath: '',
        envFilePath: '',
        winswPath: '',
        configTemplate: buildEmptyConfigTemplate(true),
        registerService: true,
        autoStart: true,
    },
    dll: {
        serviceType: 'dll',
        displayName: 'DLL Service',
        execPath: 'C:\\1Panel\\apps\\dll-service\\service.exe',
        args: '',
        workDir: 'C:\\1Panel\\apps\\dll-service',
        jarPath: '',
        dllDir: 'C:\\1Panel\\apps\\dll-service\\dll',
        configPath: '',
        envFilePath: '',
        winswPath: 'C:\\1Panel\\tools\\WinSW.exe',
        configTemplate: null,
        registerService: true,
        autoStart: true,
    },
    package: {
        serviceType: 'package',
        displayName: 'Package Service',
        execPath: '',
        args: '-Xms1024m -Xmx2048m --spring.profiles.active=prod',
        workDir: '',
        jarPath: '',
        dllDir: 'C:\\1Panel\\apps\\package\\dll',
        configPath: '',
        envFilePath: '',
        winswPath: '',
        configTemplate: buildEmptyConfigTemplate(true),
        registerService: true,
        autoStart: true,
    },
};

const isJava = computed(() => form.value.serviceType === 'java');
const isPackage = computed(() => form.value.serviceType === 'package');
const isJavaLike = computed(() => isJava.value || isPackage.value);

const dynamicConfigFields = computed(() => form.value.configTemplate?.dynamicFields ?? []);

const confirmButtonText = computed(() => {
    if (isPackage.value && packageServicePreview.value.length > 0) {
        return i18n.global.t('toolbox.windowsService.createPreviewServices', [packageServicePreview.value.length]);
    }
    return i18n.global.t('commons.button.confirm');
});

const configTemplateEnabled = computed({
    get: () => !!form.value.configTemplate?.enabled,
    set: (value: boolean) => {
        if (!isJavaLike.value) {
            form.value.configTemplate = null;
            return;
        }
        const nextTemplate = cloneConfigTemplate(form.value.configTemplate, value);
        nextTemplate.enabled = value;
        form.value.configTemplate = nextTemplate;
        if (!value) {
            form.value.configPath = '';
        }
    },
});

const deriveInstallDir = (dataDir: string) => {
    return dataDir.replace(/[\\/]+1panel$/i, '') || (isLinux.value ? '/opt' : 'C:\\1Panel');
};

// Service root under the install dir: Windows keeps <installDir>\service, Linux
// nests under the 1Panel data tree at <installDir>/1panel/services.
const serviceRootSegments = computed(() => (isLinux.value ? ['1panel', 'services'] : ['service']));

const joinPath = (...segments: string[]) => {
    const separator = isLinux.value ? '/' : '\\';
    const joined = segments.filter((segment) => segment.length > 0).join(separator);
    return isLinux.value ? joined.replace(/\/{2,}/g, '/') : joined.replace(/\\+/g, '\\');
};

const currentJarPath = computed(() => {
    if (jarFileList.value.length > 0) {
        return jarFileList.value[0].name || '';
    }
    return form.value.jarPath || '';
});

const jarTargetPath = computed(() => {
    const selectedName = jarFileList.value[0]?.name;
    if (selectedName) {
        if (!isJarPackage(selectedName)) {
            return '';
        }
        return joinPath(managedWorkDir.value, selectedName);
    }
    return form.value.jarPath || '';
});

const uploadAccept = computed(() => {
    if (form.value.serviceType === 'package') {
        return '.jar,.zip,.tar.gz,.tgz,.tar.xz,.txz';
    }
    return '.jar';
});

const managedWorkDir = computed(() => {
    if (form.value.workDir) {
        return form.value.workDir;
    }
    return joinPath(installDir.value, ...serviceRootSegments.value, form.value.name || '<service-name>', 'app');
});

const managedConfigPath = computed(() => {
    const fileName = form.value.configTemplate?.fileName || 'application-dev.yml';
    return joinPath(managedWorkDir.value, 'config', fileName);
});

const managedWinSWPath = computed(() => {
    if (form.value.winswPath) {
        return form.value.winswPath;
    }
    return joinPath(installDir.value, 'tools', 'WinSW.exe');
});

const execPathHelper = computed(() => {
    return i18n.global.t('toolbox.windowsService.execPathHelperDll');
});

const jarUploadHelper = computed(() => {
    if (form.value.serviceType === 'package') {
        return i18n.global.t('toolbox.windowsService.jarUploadHelperPackage');
    }
    return i18n.global.t('toolbox.windowsService.jarUploadHelper');
});

const workDirHelper = computed(() => {
    return i18n.global.t('toolbox.windowsService.workDirHelperDll');
});

const argsHelper = computed(() => {
    if (form.value.serviceType === 'dll') {
        return i18n.global.t('toolbox.windowsService.argsHelperDll');
    }
    if (form.value.configTemplate?.enabled) {
        return i18n.global.t('toolbox.windowsService.argsHelperTemplate');
    }
    return i18n.global.t('toolbox.windowsService.argsHelper');
});

const dllDirHelper = computed(() => {
    if (form.value.serviceType === 'package') {
        return i18n.global.t('toolbox.windowsService.dllDirHelperPackage');
    }
    return i18n.global.t('toolbox.windowsService.dllDirHelper');
});

const configPathHelper = computed(() => {
    if (form.value.serviceType === 'package') {
        return i18n.global.t('toolbox.windowsService.configPathHelperPackage');
    }
    return i18n.global.t('toolbox.windowsService.configPathHelper');
});

const winswPathHelper = computed(() => {
    if (form.value.registerService) {
        return i18n.global.t('toolbox.windowsService.winswPathHelper');
    }
    return i18n.global.t('toolbox.windowsService.winswPathHelperOptional');
});

const autoStartHelper = computed(() => {
    if (form.value.registerService) {
        return i18n.global.t('toolbox.windowsService.autoStartHelper');
    }
    return i18n.global.t('toolbox.windowsService.autoStartHelperPending');
});

const configTemplateSwitchHelper = computed(() => {
    if (form.value.serviceType === 'package') {
        return i18n.global.t('toolbox.windowsService.configTemplateSwitchHelperPackage');
    }
    return i18n.global.t('toolbox.windowsService.configTemplateSwitchHelper');
});

const buildDynamicFieldHelper = (field: WindowsServiceConfigTemplateField) => {
    return i18n.global.t('toolbox.windowsService.dynamicFieldHelper', [
        field.configPath || '-',
        field.defaultValue || i18n.global.t('toolbox.windowsService.dynamicFieldEmptyDefault'),
    ]);
};

const getConfigFieldSpan = (field: WindowsServiceConfigTemplateField) => {
    const configPath = field.configPath.toLowerCase();
    if (field.inputType === 'password' || configPath.includes('url') || configPath.includes('addresses')) {
        return 24;
    }
    return 12;
};

const validateExecPath = (_rule: unknown, value: string, callback: (error?: Error) => void) => {
    if (form.value.serviceType === 'dll' && !value.trim()) {
        callback(new Error(i18n.global.t('commons.rule.requiredInput')));
        return;
    }
    callback();
};

const validateJarUpload = (_rule: unknown, _value: string, callback: (error?: Error) => void) => {
    if (form.value.serviceType === 'dll') {
        callback();
        return;
    }
    if (form.value.serviceType === 'package' && packageTemplateDir.value.trim()) {
        callback();
        return;
    }
    if (jarFileList.value.length === 0 && !form.value.jarPath.trim()) {
        callback(
            new Error(
                form.value.serviceType === 'package'
                    ? i18n.global.t('toolbox.windowsService.jarOrTemplateRequired')
                    : i18n.global.t('toolbox.windowsService.jarRequired'),
            ),
        );
        return;
    }
    callback();
};

const validateWinSWPath = (_rule: unknown, value: string, callback: (error?: Error) => void) => {
    if (isLinux.value) {
        callback();
        return;
    }
    if (form.value.serviceType === 'dll' && form.value.registerService && !value.trim()) {
        callback(new Error(i18n.global.t('toolbox.windowsService.winswPathRequired')));
        return;
    }
    callback();
};

const rules = ref<FormRules>({
    name: [Rules.requiredInput, Rules.name],
    displayName: [Rules.requiredInput],
    serviceType: [Rules.requiredSelect],
    execPath: [{ validator: validateExecPath, trigger: 'blur' }],
    jarPath: [{ validator: validateJarUpload, trigger: 'change' }],
    winswPath: [{ validator: validateWinSWPath, trigger: 'blur' }],
});

const emit = defineEmits(['close']);

const resetJarSelection = () => {
    jarFileList.value = [];
    uploadedPackageFingerprint.value = '';
    uploadRef.value?.clearFiles();
};

const resetPackagePreview = () => {
    packageServicePreview.value = [];
};

const resetDraftState = async () => {
    selectedTemplate.value = 'java';
    draftContextKey.value = '';
    form.value = initForm();
    packageTemplateDir.value = '';
    resetPackagePreview();
    resetJarSelection();
    await nextTick();
    formRef.value?.clearValidate();
};

const handleDrawerSoftClose = () => {
    open.value = false;
};

const handleClose = async () => {
    open.value = false;
    await resetDraftState();
    emit('close');
};

const loadPlatformInfo = async () => {
    try {
        const res = await loadOsInfo();
        os.value = res.data.os || '';
        platformName.value = res.data.platform || '';
    } catch {
        os.value = '';
        platformName.value = '';
    }
};

const loadInstallDir = async () => {
    try {
        const res = await loadBaseDir();
        installDir.value = deriveInstallDir(res.data || '');
    } catch {
        installDir.value = isLinux.value ? '/opt' : 'C:\\1Panel';
    }
};

const loadConfigTemplate = async (serviceType: 'java' | 'package') => {
    const res = await getWindowsServiceConfigTemplate(serviceType);
    return cloneConfigTemplate(res.data ?? buildEmptyConfigTemplate(true), true);
};

const isJarPackage = (fileName: string) => fileName.toLowerCase().endsWith('.jar');

const isArchivePackage = (fileName: string) => {
    const lowerName = fileName.toLowerCase();
    return (
        lowerName.endsWith('.zip') ||
        lowerName.endsWith('.tar.gz') ||
        lowerName.endsWith('.tgz') ||
        lowerName.endsWith('.tar.xz') ||
        lowerName.endsWith('.txz')
    );
};

const applyTemplate = async (template: WindowsServiceTemplateType) => {
    const current = form.value;
    const nextTemplate = templateMap[template];
    if (template !== 'package' || current.serviceType !== 'package') {
        packageTemplateDir.value = '';
    }
    if (template !== current.serviceType) {
        resetPackagePreview();
    }
    form.value = {
        ...current,
        ...nextTemplate,
        id: current.id,
        name: template === 'package' ? '' : current.name,
        registerService: current.registerService,
    } as WindowsService.Update;

    if (template !== 'dll') {
        const defaultTemplate = await loadConfigTemplate(template);
        const enabled = current.configTemplate?.enabled ?? true;
        form.value.execPath = '';
        form.value.workDir = '';
        form.value.winswPath = '';
        form.value.jarPath = current.id ? current.jarPath : '';
        form.value.configTemplate = mergeConfigTemplate(defaultTemplate, current.configTemplate, enabled);
        form.value.configPath = form.value.configTemplate.enabled ? '' : form.value.configPath;
    } else {
        form.value.configTemplate = null;
    }
    resetJarSelection();
};

// Keep the service-type select in lockstep with the template select: changing it
// must reload the template and reset dependent fields, otherwise the top template
// dropdown and the dynamic config fields desync from the chosen type.
const onServiceTypeChange = (value: WindowsServiceTemplateType) => {
    selectedTemplate.value = value;
    void applyTemplate(value);
};

const beforeJarUpload: UploadProps['beforeUpload'] = (rawFile) => {
    const valid = isJarPackage(rawFile.name) || (form.value.serviceType === 'package' && isArchivePackage(rawFile.name));
    if (!valid) {
        MsgError(i18n.global.t('toolbox.windowsService.jarRequired'));
        return false;
    }
    return true;
};

const handleExceed: UploadProps['onExceed'] = (files) => {
    resetJarSelection();
    const file = files[0] as UploadRawFile;
    file.uid = genFileId();
    uploadRef.value?.handleStart(file);
};

const handleJarChange: UploadProps['onChange'] = () => {
    if (form.value.serviceType === 'package') {
        packageTemplateDir.value = '';
        resetPackagePreview();
    }
    formRef.value?.validateField('jarPath');
    void syncUploadedPackagePreview(undefined, true);
};

const handlePackageTemplateDirInput = () => {
    if (form.value.serviceType === 'package' && packageTemplateDir.value.trim()) {
        form.value.jarPath = '';
        resetJarSelection();
    }
    resetPackagePreview();
    formRef.value?.validateField('jarPath');
};

const handlePackageTemplateDirChange = () => {
    void syncPackageTemplateDirPreview();
};

const buildDraftContextKey = (item?: WindowsService.Item) => {
    if (!item) {
        return 'create';
    }
    return `edit:${item.id}:${buildConfigTemplateStructureSignature(item.configTemplate)}`;
};

const acceptParams = async (item?: WindowsService.Item) => {
    await loadPlatformInfo();
    await loadInstallDir();
    const nextContextKey = buildDraftContextKey(item);
    if (draftContextKey.value === nextContextKey) {
        open.value = true;
        return;
    }

    await resetDraftState();

    if (item) {
        let configTemplate: WindowsServiceConfigTemplate | null = null;
        if (item.serviceType !== 'dll') {
            const defaultTemplate = item.configTemplate ? null : await loadConfigTemplate(item.serviceType);
            const enabled = !!item.configTemplate?.enabled;
            const baseTemplate = item.configTemplate ?? defaultTemplate;
            configTemplate = mergeConfigTemplate(baseTemplate, item.configTemplate ?? baseTemplate, enabled);
        }

        form.value = {
            id: item.id,
            name: item.name,
            displayName: item.displayName,
            serviceType: item.serviceType,
            execPath: item.execPath,
            args: item.args,
            workDir: item.workDir,
            envFilePath: item.envFilePath,
            dllDir: item.dllDir,
            configPath: item.configPath,
            jarPath: item.jarPath,
            winswPath: item.winswPath,
            configTemplate,
            registerService: item.registerService ?? true,
            autoStart: item.autoStart,
        };
        selectedTemplate.value = item.serviceType;
        if (item.serviceType === 'package' && !item.jarPath) {
            packageTemplateDir.value = item.workDir;
            form.value.workDir = '';
        }
    } else {
        selectedTemplate.value = 'java';
        await applyTemplate('java');
    }

    draftContextKey.value = nextContextKey;
    await nextTick();
    formRef.value?.clearValidate();
    open.value = true;
};

const buildConfigTemplateStructureSignature = (template?: WindowsServiceConfigTemplate | null) => {
    if (!template) {
        return '';
    }
    return JSON.stringify({
        fileName: template.fileName || '',
        dynamicFields: (template.dynamicFields || []).map((field) => ({
            envKey: field.envKey,
            defaultValue: field.defaultValue,
            configPath: field.configPath,
            inputType: field.inputType,
            sensitive: field.sensitive,
        })),
    });
};

const buildPackageFingerprint = (
    serviceName = form.value.name,
    serviceType = form.value.serviceType,
    targetFile = jarFileList.value[0]?.raw,
) => {
    if (!targetFile) {
        return '';
    }
    const namePart = serviceType === 'package' ? 'package-bundle' : serviceName.trim();
    if (!namePart) {
        return '';
    }
    return [namePart, serviceType, targetFile.name, targetFile.size, targetFile.lastModified].join('::');
};

const applyUploadedPackageResult = (payload: WindowsService.Update, result: WindowsService.UploadJarResult) => {
    payload.jarPath = result.jarPath;
    payload.workDir = result.workDir;
    payload.execPath = result.execPath;
    payload.winswPath = result.winswPath;
    form.value.jarPath = result.jarPath;
    form.value.workDir = result.workDir;
    form.value.execPath = result.execPath;
    form.value.winswPath = result.winswPath;
    if (payload.serviceType === 'package') {
        packageServicePreview.value = result.services || [];
    }

    if (!result.configTemplate) {
        return false;
    }

    const enabled = payload.configTemplate?.enabled ?? form.value.configTemplate?.enabled ?? true;
    const currentTemplate = payload.configTemplate ?? form.value.configTemplate ?? buildEmptyConfigTemplate(enabled);
    const beforeSignature = buildConfigTemplateStructureSignature(currentTemplate);
    const mergedTemplate = mergeConfigTemplate(result.configTemplate, currentTemplate, enabled);
    mergedTemplate.content = result.configTemplate.content || mergedTemplate.content;
    payload.configTemplate = mergedTemplate;
    form.value.configTemplate = cloneConfigTemplate(mergedTemplate, enabled);
    return beforeSignature !== buildConfigTemplateStructureSignature(mergedTemplate);
};

const syncUploadedPackagePreview = async (payload?: WindowsService.Update, force = false) => {
    const targetFile = jarFileList.value[0]?.raw;
    if (!targetFile) {
        return false;
    }
    const serviceName = (payload?.name ?? form.value.name).trim();
    const serviceType = payload?.serviceType ?? form.value.serviceType;
    if (!serviceName && serviceType !== 'package') {
        return false;
    }
    const fingerprint = buildPackageFingerprint(serviceName, serviceType, targetFile);
    if (!force && fingerprint && uploadedPackageFingerprint.value === fingerprint) {
        return false;
    }
    const formData = new FormData();
    formData.append('serviceType', serviceType);
    if (serviceName) {
        formData.append('name', serviceName);
    }
    formData.append('file', targetFile);
    previewLoading.value = serviceType === 'package';
    try {
        const res = await uploadWindowsServicePackage(formData);
        uploadedPackageFingerprint.value = fingerprint;
        return applyUploadedPackageResult(payload ?? form.value, res.data);
    } finally {
        previewLoading.value = false;
    }
};

const syncPackageTemplateDirPreview = async () => {
    const templateDir = packageTemplateDir.value.trim();
    if (!isPackage.value || !templateDir || jarFileList.value.length > 0) {
        return false;
    }
    previewLoading.value = true;
    try {
        const formData = new FormData();
        formData.append('serviceType', 'package');
        formData.append('workDir', templateDir);
        const res = await uploadWindowsServicePackage(formData);
        form.value.workDir = templateDir;
        return applyUploadedPackageResult(form.value, res.data);
    } finally {
        previewLoading.value = false;
    }
};

const buildSubmitConfigTemplate = (template?: WindowsServiceConfigTemplate | null) => {
    const sourceTemplate = template ?? form.value.configTemplate;
    if (!isJavaLike.value || !sourceTemplate?.enabled) {
        return null;
    }
    const configTemplate = cloneConfigTemplate(sourceTemplate, true);
    configTemplate.generatedPath = isPackage.value ? '' : managedConfigPath.value;
    configTemplate.fileName = configTemplate.fileName || 'application-dev.yml';
    configTemplate.dynamicFields = configTemplate.dynamicFields.map((field) => ({
        ...field,
        value: field.value ?? '',
    }));
    return configTemplate;
};

const submit = async (formEl?: FormInstance) => {
    if (!formEl) return;
    await formEl.validate(async (valid) => {
        if (!valid) return;
        loading.value = true;
        try {
            const payload: WindowsService.Update = {
                ...form.value,
                configTemplate: form.value.configTemplate ? cloneConfigTemplate(form.value.configTemplate, !!form.value.configTemplate.enabled) : null,
            };
            if (isLinux.value) {
                payload.winswPath = '';
            }
            const templateDir = packageTemplateDir.value.trim();
            if (payload.serviceType === 'package' && templateDir) {
                payload.workDir = templateDir;
            }
            if (isJavaLike.value) {
                if (payload.serviceType === 'package' && templateDir && (payload.jarPath || jarFileList.value.length > 0)) {
                    MsgError(i18n.global.t('toolbox.windowsService.jarTemplateExclusive'));
                    return;
                }
                const templateChanged = await syncUploadedPackagePreview(payload);
                if (templateChanged) {
                    MsgInfo(i18n.global.t('toolbox.windowsService.templateRefreshedAfterUpload'));
                    return;
                }
                if (!payload.jarPath && !(payload.serviceType === 'package' && payload.workDir)) {
                    MsgError(
                        payload.serviceType === 'package'
                            ? i18n.global.t('toolbox.windowsService.jarOrTemplateRequired')
                            : i18n.global.t('toolbox.windowsService.jarRequired'),
                    );
                    return;
                }
            }
            payload.configTemplate = buildSubmitConfigTemplate(payload.configTemplate);
            if (payload.configTemplate?.enabled) {
                payload.configPath = managedConfigPath.value;
            }
            if (payload.serviceType === 'package') {
                payload.name = undefined;
                payload.displayName = undefined;
                payload.execPath = undefined;
                payload.args = undefined;
                payload.envFilePath = undefined;
                payload.dllDir = undefined;
                payload.configPath = undefined;
                payload.winswPath = undefined;
            }
            if (payload.id) {
                await updateWindowsService(payload);
            } else {
                const { id, ...createPayload } = payload;
                await createWindowsService(createPayload);
            }
            MsgSuccess(i18n.global.t('commons.msg.operationSuccess'));
            await handleClose();
        } finally {
            loading.value = false;
        }
    });
};

const formatRuntimeType = (service: WindowsServicePreviewItem) => {
    const runtimeType = (service.runtimeType || service.serviceType || '').toLowerCase();
    if (runtimeType.includes('compose') || service.composeServiceName) {
        return i18n.global.t('toolbox.windowsService.runtimeDockerCompose');
    }
    return i18n.global.t('toolbox.windowsService.runtimeJava');
};

defineExpose({
    acceptParams,
});
</script>
