<script setup lang="ts">
// Main application component using Composition API
// Provides the base layout structure with Vuetify app wrapper

function openGitHub() {
  window.open('https://github.com/dmarc-analyzer/dmarc-analyzer', '_blank')
}
</script>

<template>
  <v-app>
    <v-app-bar app color="primary" dark>
      <v-app-bar-title>DMARC Analyzer</v-app-bar-title>

      <v-spacer />

      <div
        class="github-link hover-effect"
        @click="openGitHub"
      >
        <img
          src="/icon.png"
          alt="DMARC Analyzer Logo"
          class="logo-image"
        >
        <span class="logo-text">
          DMARC Analyzer
        </span>
      </div>
    </v-app-bar>

    <v-main>
      <v-container fluid>
        <router-view />
      </v-container>
    </v-main>
  </v-app>
</template>

<style lang="scss">
// Global styles
.github-link {
  cursor: pointer;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 8px;
}

.logo-image {
  height: 32px;
  margin-bottom: 4px;
}

.logo-text {
  font-size: 0.7rem;
  line-height: 1rem;
  text-align: center;
}

.hover-effect {
  transition: opacity 0.2s ease;

  &:hover {
    opacity: 0.8;
  }
}
</style>
