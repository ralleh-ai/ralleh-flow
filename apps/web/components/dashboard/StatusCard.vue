<script setup lang="ts">
defineProps<{
  title: string
  value: string
  hint: string
}>()
</script>

<template>
  <section class="rf-card">
    <p class="text-xs uppercase tracking-[0.25em] text-[color:var(--rf-muted)]">{{ title }}</p>
    <p class="mt-3 text-3xl font-semibold">{{ value }}</p>
    <p class="mt-2 text-sm text-[color:var(--rf-muted)]">{{ hint }}</p>
  </section>
</template>
