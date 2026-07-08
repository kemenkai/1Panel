export namespace WindowsService {
    export interface ConfigTemplateField {
        envKey: string;
        defaultValue: string;
        value: string;
        configPath: string;
        inputType: 'text' | 'number' | 'password';
        sensitive: boolean;
    }

    export interface ConfigTemplate {
        enabled: boolean;
        template: string;
        fileName?: string;
        generatedPath?: string;
        content?: string;
        dynamicFields: ConfigTemplateField[];
    }

    export interface Item {
        id: number;
        name: string;
        displayName: string;
        serviceType: 'java' | 'dll' | 'package';
        execPath: string;
        args: string;
        workDir: string;
        envFilePath: string;
        dllDir: string;
        configPath: string;
        jarPath: string;
        winswPath: string;
        servicePath: string;
        uninstallScriptPath: string;
        status: string;
        message: string;
        configTemplate?: ConfigTemplate | null;
        registerService: boolean;
        autoStart: boolean;
    }

    export interface Create {
        name?: string;
        displayName?: string;
        serviceType: 'java' | 'dll' | 'package';
        execPath?: string;
        args?: string;
        workDir: string;
        envFilePath?: string;
        dllDir?: string;
        configPath?: string;
        jarPath?: string;
        winswPath?: string;
        configTemplate: ConfigTemplate | null;
        registerService: boolean;
        autoStart: boolean;
    }

    export interface UploadServicePreview {
        name: string;
        displayName: string;
        serviceType: 'java' | 'dll' | 'package';
        runtimeType: string;
        workDir: string;
        configPath: string;
        jarPath: string;
        execPath: string;
        args: string;
        winswPath: string;
        sourcePath: string;
        composeServiceName: string;
    }

    export interface UploadJarResult {
        jarPath: string;
        workDir: string;
        execPath: string;
        winswPath: string;
        fileName: string;
        installDir: string;
        configTemplate?: ConfigTemplate | null;
        services?: UploadServicePreview[];
    }

    export interface Update extends Create {
        id: number;
    }

    export interface Operate {
        id: number;
        operate: 'start' | 'stop' | 'restart' | 'enable' | 'disable' | 'status';
    }

    export interface ConfigFile {
        type: 'config' | 'env';
        path: string;
        content: string;
    }

    export interface ConfigFileUpdate {
        type: 'config' | 'env';
        content: string;
    }

    export interface LogFile {
        name: string;
        path: string;
        exists: boolean;
        size: number;
        modifiedAt: string;
    }
}
