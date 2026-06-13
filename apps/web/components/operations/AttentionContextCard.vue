<script setup lang="ts">
import type { OperationalFact, OperationalLink } from '~/types/ui'

const props = withDefaults(defineProps<{
  kicker?: string
  title: string
  summary: string
  facts?: OperationalFact[]
  bullets?: string[]
  links?: OperationalLink[]
}>(), {
  kicker: 'Attention context',
  facts: () => [],
  bullets: () => [],
  links: () => []
})

const toneClass = (tone?: OperationalFact['tone']) => {
  if (tone === 'ok') return 'rf-badge rf-badge--ok'
  if (tone === 'warn') return 'rf-badge rf-badge--warn'
  if (tone === 'danger') return 'rf-badge rf-badge--danger'
  return 'rf-badge'
}
</script>

<template>
  <section class="rf-card">
    <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">{{ props.kicker }}</div>
    <h2 class="mt-2 text-lg font-semibold">{{ props.title }}</h2>
    <p class="mt-3 text-sm text-[color:var(--rf-muted)]">{{ props.summary }}</p>

    <div v-if="props.facts.length" class="mt-4 flex flex-wrap gap-2">
      <span
        v-for="fact in props.facts"
        :key="`${fact.label}-${fact.value}`"
        :class="toneClass(fact.tone)"
      >
        {{ fact.label }} · {{ fact.value }}
      </span>
    </div>

    <ul v-if="props.bullets.length" class="mt-4 space-y-3 text-sm text-[color:var(--rf-muted)]">
      <li v-for="bullet in props.bullets" :key="bullet">{{ bullet }}</li>
    </ul>

    <div v-if="props.links.length" class="mt-4 flex flex-wrap gap-3">
      <NuxtLink
        v-for="link in props.links"
        :key="`${link.to}-${link.label}`"
        :to="link.to"
        class="rf-button"
      >
        {{ link.label }}
      </NuxtLink>
    </div>
  </section>
</template>
