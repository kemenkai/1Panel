<template>
    <router-view v-slot="{ Component, route }">
        <component :is="Component" :key="route.fullPath"></component>
    </router-view>
</template>

<script setup lang="ts">
defineOptions({ name: 'EnhanceRouterView' });
</script>
