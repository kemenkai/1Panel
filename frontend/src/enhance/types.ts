export namespace Enhance {
    export interface SimpleNodeItem {
        id: number;
        name: string;
        addr: string;
        description: string;
        systemVersion: string;
        securityEntrance: string;
        apiKey: string;
        cpuUsedPercent: number;
        cpuTotal: number;
        memoryTotal: number;
        memoryUsedPercent: number;
        status: string;
        message: string;
        lastCheckAt?: string;
    }

    export interface SimpleNodeCreate {
        name: string;
        addr: string;
        securityEntrance: string;
        apiKey: string;
        description: string;
    }

    export interface SimpleNodeUpdate extends SimpleNodeCreate {
        id: number;
    }
}
