<script setup lang="ts">
import DefaultTheme from 'vitepress/theme'
import ChannelBanner from './ChannelBanner.vue'

const { Layout } = DefaultTheme
</script>

<template>
  <Layout>
    <template #layout-top>
      <ChannelBanner />
    </template>
  </Layout>
</template>
