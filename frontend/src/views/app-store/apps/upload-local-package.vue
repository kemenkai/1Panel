<template>
    <el-dialog v-model="open" width="520px" :title="$t('app.uploadLocalAppPackage')" @close="handleClose">
        <el-alert class="mb-4" type="info" :closable="false" :title="$t('app.uploadLocalAppPackageHelper')" />
        <el-upload
            ref="uploadRef"
            v-model:file-list="fileList"
            drag
            :auto-upload="false"
            :limit="1"
            accept=".tar.gz"
            :on-exceed="handleExceed"
            :before-upload="beforeUpload"
            :on-change="handleChange"
        >
            <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
            <div class="el-upload__text">
                {{ $t('file.dropHelper') }}
                <em>{{ $t('file.clickHelper') }}</em>
            </div>
            <template #tip>
                <div class="el-upload__tip">{{ $t('xpack.customApp.appStoreUrlHelper') }}</div>
            </template>
        </el-upload>
        <div v-if="apps.length > 0" class="mt-4">
            <div class="font-medium mb-2">{{ $t('app.uploadLocalAppPackageDetected') }}</div>
            <el-tag v-for="item in apps" :key="item" class="mr-2 mb-2">{{ item }}</el-tag>
        </div>
        <div v-if="existingApps.length > 0" class="mt-4">
            <el-alert type="warning" :closable="false" :title="$t('app.uploadLocalAppPackageExisting')" />
            <div class="mt-2">
                <el-tag v-for="item in existingApps" :key="item" type="warning" class="mr-2 mb-2">{{ item }}</el-tag>
            </div>
        </div>
        <el-form class="mt-4">
            <el-form-item :label="$t('app.uploadLocalAppPackageStrategy')">
                <el-radio-group v-model="strategy">
                    <el-radio label="overwrite">{{ $t('app.uploadLocalAppPackageStrategyOverwrite') }}</el-radio>
                    <el-radio label="skip">{{ $t('app.uploadLocalAppPackageStrategySkip') }}</el-radio>
                    <el-radio label="fail">{{ $t('app.uploadLocalAppPackageStrategyFail') }}</el-radio>
                </el-radio-group>
            </el-form-item>
        </el-form>
        <template #footer>
            <el-button :disabled="loading" @click="open = false">{{ $t('commons.button.cancel') }}</el-button>
            <el-button type="primary" :loading="loading" @click="submit">
                {{ $t('commons.button.upload') }}
            </el-button>
        </template>
    </el-dialog>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { genFileId, type UploadInstance, type UploadProps, type UploadRawFile, type UploadUserFile } from 'element-plus';
import { UploadFilled } from '@element-plus/icons-vue';
import { previewLocalAppPackage, uploadLocalAppPackage } from '@/api/modules/app';
import { MsgError } from '@/utils/message';
import { newUUID } from '@/utils/id';
import i18n from '@/lang';

const open = ref(false);
const loading = ref(false);
const strategy = ref<'overwrite' | 'skip' | 'fail'>('overwrite');
const fileList = ref<UploadUserFile[]>([]);
const uploadRef = ref<UploadInstance>();
const apps = ref<string[]>([]);
const existingApps = ref<string[]>([]);

const emit = defineEmits<{
    uploaded: [payload: { taskID: string; apps: string[] }];
}>();

const beforeUpload: UploadProps['beforeUpload'] = (rawFile) => {
    const isTarGz = rawFile.name.toLowerCase().endsWith('.tar.gz');
    if (!isTarGz) {
        MsgError(i18n.global.t('app.uploadLocalAppPackageFormatError'));
        return false;
    }
    return true;
};

const handleExceed: UploadProps['onExceed'] = (files) => {
    uploadRef.value?.clearFiles();
    const file = files[0] as UploadRawFile;
    file.uid = genFileId();
    uploadRef.value?.handleStart(file);
};

const handleChange: UploadProps['onChange'] = async (file) => {
    const raw = file.raw;
    if (!raw) {
        return;
    }
    const isTarGz = raw.name.toLowerCase().endsWith('.tar.gz');
    if (!isTarGz) {
        return;
    }
    loading.value = true;
    try {
        const formData = new FormData();
        formData.append('file', raw);
        const res = await previewLocalAppPackage(formData);
        apps.value = res.data.apps || [];
        existingApps.value = res.data.existingApps || [];
        if (existingApps.value.length > 0 && strategy.value === 'fail') {
            strategy.value = 'overwrite';
        }
    } finally {
        loading.value = false;
    }
};

const submit = async () => {
    const targetFile = fileList.value[0]?.raw;
    if (!targetFile) {
        MsgError(i18n.global.t('app.uploadLocalAppPackageSelect'));
        return;
    }
    loading.value = true;
    try {
        const taskID = newUUID();
        const formData = new FormData();
        formData.append('file', targetFile);
        formData.append('taskID', taskID);
        formData.append('strategy', strategy.value);
        const res = await uploadLocalAppPackage(formData);
        open.value = false;
        fileList.value = [];
        emit('uploaded', { taskID: res.data.taskID || taskID, apps: res.data.apps || [] });
    } finally {
        loading.value = false;
    }
};

const handleClose = () => {
    fileList.value = [];
    apps.value = [];
    existingApps.value = [];
    strategy.value = 'overwrite';
};

const acceptParams = () => {
    fileList.value = [];
    apps.value = [];
    existingApps.value = [];
    strategy.value = 'overwrite';
    open.value = true;
};

defineExpose({
    acceptParams,
});
</script>
