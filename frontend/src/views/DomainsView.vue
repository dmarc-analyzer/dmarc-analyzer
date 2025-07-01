<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useDmarcStore } from '@/stores/dmarcStore'
import { calculatePassingPercentage, formatNumber, getlast30DayRange, getPercentageColor, getSessionStorage, removeSessionStorage, setSessionStorage } from '@/utils/utilities'

// Initialize composables
const router = useRouter()
const dmarcStore = useDmarcStore()

// Reactive state
const filterText = ref('')

// Computed properties

/**
 * Filter domains based on search text with case-insensitive matching
 */
const filteredDomains = computed(() => {
  if (!dmarcStore.domains)
    return []

  if (!filterText.value) {
    return dmarcStore.domains
  }

  // Case-insensitive domain filtering
  const query = filterText.value.toLowerCase()
  return dmarcStore.domains.filter(domainStat =>
    domainStat.domain.toLowerCase().includes(query),
  )
})

// Methods

/**
 * Handle search input changes and persist filter state
 * @param value - Current filter text value
 */
function onFilter(value: string) {
  filterText.value = value || ''

  // Persist filter state to session storage for restoration when returning
  if (filterText.value && filterText.value.length > 0) {
    setSessionStorage('filterString', filterText.value)
  }
  else {
    removeSessionStorage('filterString')
  }
}

/**
 * Navigate to domain report view with default date range
 * @param domain - Domain name to view reports for
 */
function goToReport(domain: string) {
  // Store current filter for restoration when returning
  if (filterText.value) {
    setSessionStorage('filterString', filterText.value)
  }

  // Get default date range (last 30 days)
  const dateRange = getlast30DayRange()

  // Navigate to report view with domain and date parameters
  router.push({
    name: 'ReportWithDates',
    params: {
      domain,
      start: dateRange.startDate,
      end: dateRange.endDate,
    },
  })
}

// Lifecycle hooks

/**
 * Component mounted - fetch domains and restore filter state
 */
onMounted(() => {
  // Fetch domains from API
  dmarcStore.fetchDomains()

  // Restore filter state from session storage
  const storedFilter = getSessionStorage('filterString')
  if (storedFilter && storedFilter.length > 0) {
    filterText.value = storedFilter
  }
})
</script>

<template>
  <div class="domains-view">
    <!-- Page title section -->
    <div class="page-header mb-4">
      <h1 class="text-h4 mb-2">
        Domains
      </h1>
      <div class="text-subtitle-1 text-medium-emphasis">
        DMARC domain statistics for the last 30 days
      </div>
    </div>

    <!-- Search input section using native Vuetify component -->
    <div class="search-section mb-4">
      <v-text-field
        v-model="filterText"
        label="Search your domains"
        prepend-inner-icon="mdi-magnify"
        variant="outlined"
        color="primary"
        density="comfortable"
        clearable
        hide-details
        class="search-input"
        style="max-width: 400px;"
        @update:model-value="onFilter"
        @keyup.enter="onFilter"
      />
    </div>

    <v-divider class="mb-4" />

    <!-- Content area with loading, error, and data states -->
    <div class="content-area">
      <!-- Loading state using native Vuetify progress -->
      <div v-if="dmarcStore.loading" class="loading-container">
        <v-progress-circular
          indeterminate
          color="primary"
          size="40"
          width="2"
        />
      </div>

      <!-- Error state using native Vuetify alert -->
      <v-alert
        v-if="dmarcStore.error"
        type="error"
        variant="tonal"
        class="mb-4"
      >
        {{ dmarcStore.error }}
      </v-alert>

      <!-- Empty state message -->
      <div
        v-if="!dmarcStore.loading && !dmarcStore.error && filteredDomains.length === 0"
        class="no-domains-container"
      >
        <v-card class="pa-8 text-center">
          <v-icon size="64" color="grey-lighten-1" class="mb-4">
            mdi-domain-off
          </v-icon>
          <h4 class="text-h6 mb-2">
            <span v-if="!filterText">No Domains Available</span>
            <span v-else>No domains found matching "{{ filterText }}"</span>
          </h4>
          <p v-if="!filterText" class="text-body-2 text-medium-emphasis">
            Get started by adding a domain to analyze DMARC reports.
          </p>
        </v-card>
      </div>

      <!-- Domains grid using responsive layout -->
      <div v-if="filteredDomains.length > 0" class="domains-grid">
        <v-card
          v-for="domainStat in filteredDomains"
          :key="domainStat.domain"
          class="domain-card"
          elevation="2"
          hover
          @click="goToReport(domainStat.domain)"
        >
          <v-card-text class="domain-card-content">
            <!-- Domain security icon -->
            <div class="domain-icon">
              <v-icon size="24" color="primary">
                mdi-security
              </v-icon>
            </div>

            <!-- Domain info section -->
            <div class="domain-info">
              <!-- Domain name -->
              <div class="domain-name" :title="domainStat.domain">
                {{ domainStat.domain }}
              </div>

              <!-- Statistics summary -->
              <div class="domain-stats">
                <div class="stats-row">
                  <span class="stats-label">Total Messages:</span>
                  <span class="stats-value">{{ formatNumber(domainStat.total_count) }}</span>
                </div>
                <div class="stats-row">
                  <span class="stats-label">Passed:</span>
                  <span class="stats-value">{{ formatNumber(domainStat.pass_count) }}</span>
                </div>
                <div class="stats-row">
                  <span class="stats-label">Passing %:</span>
                  <v-chip
                    :color="getPercentageColor(calculatePassingPercentage(domainStat))"
                    size="small"
                    :variant="calculatePassingPercentage(domainStat) === 0 ? 'flat' : 'tonal'"
                  >
                    {{ calculatePassingPercentage(domainStat).toFixed(2) }}%
                  </v-chip>
                </div>
              </div>
            </div>

            <!-- Action button -->
            <div class="domain-actions">
              <v-btn
                icon
                size="small"
                color="primary"
                variant="text"
                :aria-label="`View DMARC report for ${domainStat.domain}`"
                @click.stop="goToReport(domainStat.domain)"
              >
                <v-icon size="20">
                  mdi-chart-line
                </v-icon>
              </v-btn>
            </div>
          </v-card-text>
        </v-card>
      </div>
    </div>
  </div>
</template>

<style lang="scss" scoped>
.domains-view {
  // Page header styling
  .page-header {
    .text-h4 {
      font-weight: 500;
      color: rgba(0, 0, 0, 0.87);
    }
  }

  // Search section responsive behavior
  .search-section {
    .search-input {
      // Responsive width adjustment
      @media (max-width: 599px) {
        max-width: none !important;
        width: 100%;
      }
    }
  }

  // Content area layout
  .content-area {
    position: relative;
  }

  // Loading state centered container
  .loading-container {
    display: flex;
    justify-content: center;
    align-items: center;
    min-height: 200px;
  }

  // Empty state styling
  .no-domains-container {
    display: flex;
    justify-content: center;
    align-items: center;
    min-height: 300px;
  }

  // Responsive domains grid
  .domains-grid {
    display: grid;
    gap: 16px;
    padding: 8px 0;

    // Responsive grid columns - wider cards for better domain display
    grid-template-columns: repeat(auto-fill, minmax(380px, 1fr));

    @media (max-width: 599px) {
      grid-template-columns: 1fr;
    }

    @media (min-width: 600px) and (max-width: 959px) {
      grid-template-columns: repeat(auto-fill, minmax(350px, 1fr));
    }

    @media (min-width: 960px) and (max-width: 1263px) {
      grid-template-columns: repeat(auto-fill, minmax(380px, 1fr));
    }

    @media (min-width: 1264px) {
      grid-template-columns: repeat(auto-fill, minmax(420px, 1fr));
    }
  }

  // Domain card styling
  .domain-card {
    cursor: pointer;
    transition: all 0.2s cubic-bezier(0.25, 0.8, 0.25, 1);
    height: 100%;

    // Hover effects
    &:hover {
      transform: translateY(-2px);
      // Enhanced shadow on hover
      box-shadow: 0 8px 25px rgba(0, 0, 0, 0.15) !important;
    }

    .domain-card-content {
      display: flex;
      align-items: flex-start;
      gap: 16px;
      padding: 16px !important;
      min-height: 120px;

      .domain-icon {
        flex-shrink: 0;
        margin-top: 4px;
      }

      .domain-info {
        flex: 1;
        min-width: 0;

        .domain-name {
          font-size: 1.1rem;
          font-weight: 500;
          color: rgba(0, 0, 0, 0.87);
          overflow: hidden;
          text-overflow: ellipsis;
          white-space: nowrap;
          margin-bottom: 12px;

          // Responsive font size
          @media (max-width: 599px) {
            font-size: 1rem;
          }
        }

        .domain-stats {
          display: flex;
          flex-direction: column;
          gap: 6px;

          .stats-row {
            display: flex;
            justify-content: space-between;
            align-items: center;
            font-size: 0.875rem;

            .stats-label {
              color: rgba(0, 0, 0, 0.6);
              font-weight: 500;
            }

            .stats-value {
              color: rgba(0, 0, 0, 0.87);
              font-weight: 600;
            }
          }
        }
      }

      .domain-actions {
        flex-shrink: 0;

        .v-btn {
          transition: all 0.2s;

          &:hover {
            background-color: rgba(25, 118, 210, 0.08);
            transform: scale(1.1);
          }
        }
      }
    }
  }
}

// Global divider styling
:deep(.v-divider) {
  margin: 24px 0;

  &.section {
    margin: 16px 0;
  }
}
</style>
