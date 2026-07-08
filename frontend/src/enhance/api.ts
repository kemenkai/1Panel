import http from '@/api';
import type { WindowsService } from '@/api/interface/windows-service';
import type { Enhance } from './types';

const buildNodeHeaders = (node?: string) => {
    if (!node) {
        return undefined;
    }
    return {
        CurrentNode: node,
    };
};

export const listAllSimpleNodes = () => {
    return http.get<Array<Enhance.SimpleNodeItem>>(`/core/nodes/simple/all`);
};

export const createSimpleNode = (params: Enhance.SimpleNodeCreate) => {
    return http.post(`/core/xpack/nodes/simple`, params);
};

export const updateSimpleNode = (params: Enhance.SimpleNodeUpdate) => {
    return http.post(`/core/xpack/nodes/simple/update/base`, params);
};

export const deleteSimpleNode = (id: number) => {
    return http.post(`/core/xpack/nodes/simple/del`, { ids: [id] });
};

export const checkSimpleNode = (params: Partial<Enhance.SimpleNodeUpdate>) => {
    return http.post<Enhance.SimpleNodeItem>(`/core/xpack/nodes/simple/check`, params);
};

export const refreshSimpleNode = (id: number) => {
    return http.post<Enhance.SimpleNodeItem>(`/core/xpack/nodes/simple/refresh`, { id });
};

export const refreshAllSimpleNodes = () => {
    return http.post<Array<Enhance.SimpleNodeItem>>(`/core/xpack/nodes/simple/refresh/all`);
};

export const buildSimpleNodeVisitURL = (id: number, redirect = '/') => {
    return http.post<string>(`/core/xpack/nodes/simple/visit`, { id, redirect });
};

export const listWindowsServices = (node?: string) => {
    return http.get<Array<WindowsService.Item>>(`/windows/services`, {}, { headers: buildNodeHeaders(node) });
};

export const getWindowsServiceConfigFile = (id: number, type: 'config' | 'env', node?: string) => {
    return http.get<WindowsService.ConfigFile>(
        `/windows/services/${id}/config-file`,
        { type },
        { headers: buildNodeHeaders(node) },
    );
};

export const updateWindowsServiceConfigFile = (id: number, params: WindowsService.ConfigFileUpdate, node?: string) => {
    return http.post(`/windows/services/${id}/config-file`, params, undefined, buildNodeHeaders(node));
};

export const getWindowsServiceLogFile = (id: number, node?: string) => {
    return http.get<WindowsService.LogFile>(`/windows/services/${id}/log`, {}, { headers: buildNodeHeaders(node) });
};

export const createWindowsService = (params: WindowsService.Create, node?: string) => {
    return http.post(`/windows/services`, params, undefined, buildNodeHeaders(node));
};

export const uploadWindowsServiceJar = (params: FormData, node?: string) => {
    return http.upload<WindowsService.UploadJarResult>(`/windows/services/upload-jar`, params, {
        headers: {
            'Content-Type': 'multipart/form-data',
            ...buildNodeHeaders(node),
        },
    });
};

export const updateWindowsService = (params: WindowsService.Update, node?: string) => {
    return http.post(`/windows/services/update`, params, undefined, buildNodeHeaders(node));
};

export const operateWindowsService = (params: WindowsService.Operate, node?: string) => {
    return http.post<WindowsService.Item>(`/windows/services/operate`, params, undefined, buildNodeHeaders(node));
};

export const deleteWindowsService = (id: number, node?: string) => {
    return http.post(`/windows/services/del`, { id }, undefined, buildNodeHeaders(node));
};
