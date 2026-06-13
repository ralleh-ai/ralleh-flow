<script setup lang="ts">
import type { OperationalFact } from '~/types/ui'

const props = withDefaults(defineProps<{
  facts: OperationalFact[]
  columns?: 2 | 3 | 4
}>(), {
  columns: 4
})

const columnsClass = computed(() => {
  if (props.columns === 2) return 'md:grid-cols-2'
  if (props.columns === 3) return 'md:grid-cols-2 xl:grid-cols-3'
  return 'md:grid-cols-2 xl:grid-cols-4'
})

const valueToneClass = (tone?: OperationalFact['tone']) => {
  if (tone === 'ok') return 'text-emerald-200'
  if (tone === 'warn') return 'text-amber-200'
  if (tone === 'danger') return 'text-rose-200'
  return 'text-white/90'
}
</script>

<template>
  <div v-if="props.facts.length" :class="['grid gap-3', columnsClass]">
    <div
      v-for="fact in props.facts"
      :key="`${fact.label}-${fact.value}-${fact.detail || ''}`"
      class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-3"
    >
      <div class="text-xs uppercase tracking-[0.2em] text-[color:var(--rf-muted)]">{{ fact.label }}</div>
      <div class="mt-2 text-sm font-medium" :class="valueToneClass(fact.tone)">{{ fact.value }}</div>
      <p v-if="fact.detail" class="mt-2 text-xs text-[color:var(--rf-muted)]">{{ fact.detail }}</p>
    </div>
  </div>
</template>
