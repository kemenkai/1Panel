import { ref, computed } from 'vue';
import { loadOsInfo } from '@/api/modules/dashboard';

// Windows Lite panel keeps only these top-level menus in the sidebar; every
// other entry is Linux-only and must stay hidden. Keep this list in sync with
// filterWindowsLiteMenus in core/app/service/setting.go.
export const WINDOWS_ALLOWED_TOP_MENUS = [
    'Home-Menu',
    'Enhance-Menu',
    'Container-Menu',
    'Toolbox-Menu',
    'Log-Menu',
    'Setting-Menu',
];

const osInfo = ref({ os: '', platform: '', platformFamily: '' });
const isOsLoaded = ref(false);
let inflight: Promise<void> | null = null;

const isWindowsPanel = computed(() => {
    const merged = `${osInfo.value.os} ${osInfo.value.platform} ${osInfo.value.platformFamily}`.toLowerCase();
    return merged.includes('windows');
});

// ensureOsInfo loads the OS info once and shares the same request across all
// callers. Safe to await repeatedly; it resolves immediately once loaded.
async function ensureOsInfo(): Promise<void> {
    if (isOsLoaded.value) {
        return;
    }
    if (!inflight) {
        inflight = loadOsInfo()
            .then((res) => {
                osInfo.value = {
                    os: res.data.os || '',
                    platform: res.data.platform || '',
                    platformFamily: res.data.platformFamily || '',
                };
            })
            .catch(() => {
                // On failure treat the panel as non-Windows (full menu),
                // matching the previous fallback behavior.
            })
            .finally(() => {
                isOsLoaded.value = true;
                inflight = null;
            });
    }
    await inflight;
}

export function useWindowsPanel() {
    return { isWindowsPanel, isOsLoaded, ensureOsInfo, osInfo };
}
