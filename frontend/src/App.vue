<template>
  <div class="workbench-shell">
    <aside class="sidebar">
      <div class="brand">
        <strong>Task Agent Workbench</strong>
        <span>Interview training system</span>
      </div>

      <nav class="nav-list" aria-label="Primary">
        <RouterLink
          v-for="item in navItems"
          :key="item.to"
          :to="item.to"
          class="nav-link"
          :class="{ exact: item.exact }"
        >
          {{ item.label }}
        </RouterLink>
      </nav>
    </aside>

    <main class="content-shell">
      <header class="workbench-topbar">
        <div class="user-control">
          <label for="global-user-id">user_id</label>
          <input id="global-user-id" :value="state.userId" @input="setUserId($event.target.value)" />
        </div>

        <div class="selected-ids" aria-label="Selected workbench ids">
          <span>interview: <strong>{{ state.selectedInterviewId || '-' }}</strong></span>
          <span>plan: <strong>{{ state.selectedPlanId || '-' }}</strong></span>
          <span>session: <strong>{{ state.selectedSessionId || '-' }}</strong></span>
          <span>mock: <strong>{{ state.selectedMockId || '-' }}</strong></span>
        </div>

        <div class="runtime-status">
          <span v-if="loadingLabels.length">Running: {{ loadingLabels.join(', ') }}</span>
          <span v-else>API proxy: /api</span>
        </div>
      </header>

      <ErrorNotice v-if="state.globalError" :message="state.globalError" />
      <p v-if="state.lastResult" class="notice success">{{ state.lastResult }}</p>

      <RouterView />
    </main>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { RouterLink, RouterView } from 'vue-router'
import ErrorNotice from './components/ErrorNotice.vue'
import { navItems } from './router'
import { setUserId, workbenchState as state } from './workbenchState'

const loadingLabels = computed(() => Array.from(state.loadingKeys))
</script>

<style scoped>
.workbench-shell {
  background: #f4f6f8;
  color: #172033;
  display: grid;
  grid-template-columns: 244px minmax(0, 1fr);
  min-height: 100vh;
}

.sidebar {
  background: #ffffff;
  border-right: 1px solid #d7dde6;
  display: flex;
  flex-direction: column;
  gap: 22px;
  padding: 22px 16px;
}

.brand {
  display: grid;
  gap: 4px;
}

.brand strong {
  font-size: 17px;
}

.brand span,
.selected-ids,
.runtime-status {
  color: #667286;
  font-size: 12px;
}

.nav-list {
  display: grid;
  gap: 6px;
}

.nav-link {
  border-radius: 6px;
  color: #3a4658;
  font-weight: 700;
  padding: 9px 10px;
  text-decoration: none;
}

.nav-link:hover,
.nav-link.router-link-active {
  background: #eef2f7;
  color: #172033;
}

.nav-link.exact.router-link-active:not(.router-link-exact-active) {
  background: transparent;
  color: #3a4658;
}

.content-shell {
  display: grid;
  gap: 14px;
  grid-template-rows: auto auto auto 1fr;
  min-width: 0;
  padding: 18px;
}

.workbench-topbar {
  align-items: center;
  background: #ffffff;
  border: 1px solid #d7dde6;
  border-radius: 8px;
  display: grid;
  gap: 14px;
  grid-template-columns: minmax(180px, 260px) minmax(0, 1fr) auto;
  padding: 14px;
}

.user-control {
  display: grid;
  gap: 5px;
}

.user-control label {
  color: #4d596b;
  font-size: 12px;
  font-weight: 700;
}

.selected-ids {
  display: grid;
  gap: 4px 12px;
  grid-template-columns: repeat(4, minmax(0, 1fr));
}

.selected-ids span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.runtime-status {
  text-align: right;
}

:deep(.page) {
  background: #ffffff;
  border: 1px solid #d7dde6;
  border-radius: 8px;
  display: grid;
  gap: 14px;
  padding: 18px;
}

:deep(.page-header) {
  align-items: flex-start;
  display: flex;
  gap: 16px;
  justify-content: space-between;
}

:deep(.page-kicker) {
  color: #667286;
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
}

:deep(.stub-grid) {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
}

@media (max-width: 900px) {
  .workbench-shell {
    grid-template-columns: 1fr;
  }

  .sidebar {
    border-bottom: 1px solid #d7dde6;
    border-right: 0;
  }

  .nav-list {
    grid-template-columns: repeat(auto-fit, minmax(130px, 1fr));
  }

  .workbench-topbar,
  .selected-ids {
    grid-template-columns: 1fr;
  }

  .runtime-status {
    text-align: left;
  }
}
</style>
