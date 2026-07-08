import http from '@/api';
import { WindowsService } from '../interface/windows-service';

export const listWindowsServices = () => {
    return http.get<Array<WindowsService.Item>>(`/windows/services`);
};

export const getWindowsServiceConfigTemplate = (serviceType: 'java' | 'package' | 'dll') => {
    return http.get<WindowsService.ConfigTemplate | null>(`/windows/services/config-template`, { serviceType });
};

export const getWindowsServiceConfigFile = (id: number, type: 'config' | 'env') => {
    return http.get<WindowsService.ConfigFile>(`/windows/services/${id}/config-file`, { type });
};

export const updateWindowsServiceConfigFile = (id: number, params: WindowsService.ConfigFileUpdate) => {
    return http.post(`/windows/services/${id}/config-file`, params);
};

export const getWindowsServiceLogFile = (id: number) => {
    return http.get<WindowsService.LogFile>(`/windows/services/${id}/log`);
};

export const createWindowsService = (params: WindowsService.Create) => {
    return http.post(`/windows/services`, params);
};

export const uploadWindowsServicePackage = (params: FormData) => {
    return http.upload<WindowsService.UploadJarResult>(`/windows/services/upload-package`, params, {
        headers: { 'Content-Type': 'multipart/form-data' },
    });
};

export const uploadWindowsServiceJar = uploadWindowsServicePackage;

export const updateWindowsService = (params: WindowsService.Update) => {
    return http.post(`/windows/services/update`, params);
};

export const operateWindowsService = (params: WindowsService.Operate) => {
    return http.post<WindowsService.Item>(`/windows/services/operate`, params);
};

export const deleteWindowsService = (id: number) => {
    return http.post(`/windows/services/del`, { id });
};
