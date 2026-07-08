<template>
    <router-view v-slot="{ Component, route }" :key="key">
        <component v-if="isEnhanceRoute(route.path)" :is="Component" :key="route.fullPath"></component>
        <transition v-else appear name="fade-transform" mode="out-in">
            <keep-alive :include="include">
                <component :is="Component" :key="route.path"></component>
            </keep-alive>
        </transition>
    </router-view>
</template>

<script setup lang="ts">
import cacheRouter from '@/routers/cache-router';
import { computed } from 'vue';

const key = computed(() => {
    return Math.random();
});
const include = computed(() => {
    return (props.keepAlive || cacheRouter) as string[];
});
const isEnhanceRoute = (path: string) => path.startsWith('/enhance');
const props = defineProps({
    keepAlive: {
        type: Object,
        required: false,
    },
});
</script>
