<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import DateRange from '@/components/DateRange.vue'
import DetailDialog from '@/components/DetailDialog.vue'
import LineChart from '@/components/LineChart.vue'
import { useDmarcStore } from '@/stores/dmarcStore'
import { calculatePassingPercentage, formatNumber, getlast30DayRange, getPercentageColor } from '@/utils/utilities'

// Initialize composables
const route = useRoute()
const router = useRouter()
const dmarcStore = useDmarcStore()

// Reactive state
const domain = ref('')
const startDate = ref<string | null>(null)
const endDate = ref<string | null>(null)
const detailDialog = ref(false)
const selectedSource = ref('')

// Data table headers configuration
const sourceHeaders = [
  { title: 'Source', key: 'source', sortable: false, width: '200px' },
  { title: 'Total', key: 'total_count', sortable: false, width: '120px' },
  { title: 'Passing %', key: 'passing_percentage', sortable: false, width: '120px' },
  { title: 'Pass Both', key: 'fully_aligned_count', sortable: false, width: '120px' },
  { title: 'Pass SPF', key: 'spf_aligned_count', sortable: false, width: '120px' },
  { title: 'Pass DKIM', key: 'dkim_aligned_count', sortable: false, width: '120px' },
]

// Computed properties

/**
 * Check if report has any data to display
 */
const hasData = computed(() => {
  return dmarcStore.summaryReport
    && dmarcStore.summaryReport.summary
    && dmarcStore.summaryReport.summary.length > 0
})

/**
 * Check if chart data is available and valid
 */
const hasChartData = computed(() => {
  return dmarcStore.chartData
    && dmarcStore.chartData.labels
    && Array.isArray(dmarcStore.chartData.labels)
    && dmarcStore.chartData.labels.length > 0
    && dmarcStore.chartData.datasets
    && Array.isArray(dmarcStore.chartData.datasets)
    && dmarcStore.chartData.datasets.length > 0
})

/**
 * Get domain summary counts for display
 */
const summaryCounts = computed(() => {
  return dmarcStore.summaryReport?.domain_summary_counts
})

/**
 * Transform summary data for table display
 */
const sourceItems = computed(() => {
  return dmarcStore.summaryReport?.summary ?? []
})

// Methods

/**
 * Handle row click to show detail dialog
 * @param _event - Click event (unused, prefixed with underscore)
 * @param row - Table row data containing source information
 * @param row.item - The item data from the clicked row
 */
function showDetail(_event: any, { item }: any) {
  selectedSource.value = item.source
  detailDialog.value = true

  // Fetch detailed report data for the selected source
  const sourceType = item.source_type || ''
  dmarcStore.fetchDetailReport(domain.value, item.source, startDate.value || undefined, endDate.value || undefined, sourceType)
}

/**
 * Navigate back to domains list
 */
function goBack() {
  router.push({ name: 'Domains' })
}

/**
 * Handle date range updates from DateRange component
 * @param dateRange - New date range object with startDate and endDate
 * @param dateRange.startDate - Start date in YYYY-MM-DD format
 * @param dateRange.endDate - End date in YYYY-MM-DD format
 */
function updateDateRange({ startDate: newStartDate, endDate: newEndDate }: { startDate: string, endDate: string }) {
  // Convert date strings to full ISO timestamps
  // Start date: beginning of day (00:00:00.000Z)
  // End date: end of day (23:59:59.999Z)
  const startDateTime = new Date(`${newStartDate}T00:00:00.000Z`).toISOString()
  const endDateTime = new Date(`${newEndDate}T23:59:59.999Z`).toISOString()

  startDate.value = startDateTime
  endDate.value = endDateTime

  // Update URL with new date range
  router.push({
    name: 'ReportWithDates',
    params: {
      domain: domain.value,
      start: startDateTime,
      end: endDateTime,
    },
  })

  // Fetch new data with updated date range
  dmarcStore.fetchSummaryReport(domain.value, startDate.value, endDate.value)
}

// Lifecycle hooks

/**
 * Component mounted - initialize data and fetch reports
 */
onMounted(() => {
  // Get domain from route parameters
  domain.value = route.params.domain as string

  // Check if date parameters exist in URL
  if (route.params.start && route.params.end) {
    // Use dates from URL parameters
    startDate.value = decodeURIComponent(route.params.start as string)
    endDate.value = decodeURIComponent(route.params.end as string)
  }
  else {
    // Set default date range (last 30 days)
    const dateRange = getlast30DayRange()
    startDate.value = dateRange.startDate
    endDate.value = dateRange.endDate
  }

  // Fetch initial report data
  dmarcStore.fetchSummaryReport(domain.value, startDate.value, endDate.value)
})

// Watch for route parameter changes
watch(() => route.params, (newParams) => {
  // Handle domain changes
  if (newParams.domain && newParams.domain !== domain.value) {
    domain.value = newParams.domain as string
  }

  // Handle date parameter changes
  if (newParams.start && newParams.end) {
    const newStartDate = decodeURIComponent(newParams.start as string)
    const newEndDate = decodeURIComponent(newParams.end as string)

    if (newStartDate !== startDate.value || newEndDate !== endDate.value) {
      startDate.value = newStartDate
      endDate.value = newEndDate
    }
  }

  // Fetch data with current parameters
  if (domain.value && startDate.value && endDate.value) {
    dmarcStore.fetchSummaryReport(domain.value, startDate.value, endDate.value)
  }
}, { deep: true })
</script>

<template>
  <div class="report-view">
    <!-- Page header with navigation and title -->
    <div class="page-header mb-4">
      <v-row align="center">
        <!-- Back navigation button -->
        <v-col cols="auto">
          <v-btn
            icon
            aria-label="Go back to domains list"
            @click="goBack"
          >
            <v-icon>mdi-arrow-left</v-icon>
          </v-btn>
        </v-col>

        <!-- Page title and subtitle -->
        <v-col>
          <h1 class="text-h4 mb-0 page-title">
            {{ domain }}
          </h1>
          <div class="text-subtitle-1 text-medium-emphasis">
            DMARC Report
          </div>
        </v-col>
      </v-row>
    </div>

    <!-- Date range selector card -->
    <v-card class="mb-4" elevation="1">
      <v-card-text>
        <DateRange
          :initial-start-date="startDate ? startDate.split('T')[0] : null"
          :initial-end-date="endDate ? endDate.split('T')[0] : null"
          @update:date-range="updateDateRange"
        />
      </v-card-text>
    </v-card>

    <!-- Loading indicator using native Vuetify progress -->
    <v-progress-linear
      v-if="dmarcStore.loading"
      indeterminate
      color="primary"
      class="mb-4"
    />

    <!-- Error message using native Vuetify alert -->
    <v-alert
      v-if="dmarcStore.error"
      type="error"
      variant="tonal"
      class="mb-4"
    >
      {{ dmarcStore.error }}
    </v-alert>

    <!-- No records found state -->
    <v-card
      v-if="!dmarcStore.loading && !dmarcStore.error && !hasData"
      class="mb-4"
      elevation="1"
    >
      <v-card-text class="text-center pa-8">
        <v-icon size="64" color="grey-lighten-1" class="mb-4">
          mdi-chart-line-variant
        </v-icon>
        <div class="text-h6 mb-2">
          No Records Found
        </div>
        <div class="text-body-2 text-medium-emphasis">
          No DMARC data found for domain "{{ domain }}" in the selected time range.
        </div>
      </v-card-text>
    </v-card>

    <!-- Chart visualization card -->
    <v-card
      v-if="hasChartData && dmarcStore.chartData"
      class="mb-4"
      elevation="1"
    >
      <v-card-text>
        <div style="height: 300px">
          <LineChart :chart-data="dmarcStore.chartData" />
        </div>
      </v-card-text>
    </v-card>

    <!-- Summary statistics card -->
    <v-card
      v-if="summaryCounts?.total_count"
      class="mb-4"
      elevation="1"
    >
      <v-card-text>
        <!-- Native Vuetify table for summary data -->
        <v-table density="comfortable">
          <thead>
            <tr>
              <th class="text-left font-weight-bold" />
              <th class="text-left font-weight-bold">
                Total
              </th>
              <th class="text-left font-weight-bold">
                Passing %
              </th>
              <th class="text-left font-weight-bold">
                Pass Both
              </th>
              <th class="text-left font-weight-bold">
                Pass SPF
              </th>
              <th class="text-left font-weight-bold">
                Pass DKIM
              </th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="summaryCounts.total_count">
              <td class="font-weight-bold">
                {{ domain }}
              </td>
              <td>{{ formatNumber(summaryCounts.total_count) }}</td>
              <td>
                <v-chip
                  :color="getPercentageColor(calculatePassingPercentage(summaryCounts))"
                  size="small"
                  :variant="calculatePassingPercentage(summaryCounts) === 0 ? 'flat' : 'tonal'"
                >
                  {{ (calculatePassingPercentage(summaryCounts)).toFixed(2) }}%
                </v-chip>
              </td>
              <td>{{ formatNumber(summaryCounts.fully_aligned_count) }}</td>
              <td>{{ formatNumber(summaryCounts.spf_aligned_count) }}</td>
              <td>{{ formatNumber(summaryCounts.dkim_aligned_count) }}</td>
            </tr>
          </tbody>
        </v-table>
      </v-card-text>
    </v-card>

    <!-- Sources detail table card -->
    <v-card v-if="summaryCounts?.total_count" elevation="1">
      <v-card-text>
        <!-- Native Vuetify data table with enhanced features -->
        <v-data-table
          :headers="sourceHeaders"
          :items="sourceItems"
          :items-per-page="100"
          :items-per-page-options="[100, 200, 500]"
          class="elevation-0 sources-table"
          hover
          @click:row="showDetail"
        >
          <!-- Custom percentage column with color coding -->
          <template #[`item.passing_percentage`]="{ item }">
            <v-chip
              :color="getPercentageColor(calculatePassingPercentage(item))"
              size="small"
              :variant="calculatePassingPercentage(item) === 0 ? 'flat' : 'tonal'"
            >
              {{ (calculatePassingPercentage(item)).toFixed(2) }}%
            </v-chip>
          </template>

          <!-- Formatted number columns -->
          <template #[`item.total_count`]="{ item }">
            {{ formatNumber(item.total_count) }}
          </template>

          <template #[`item.fully_aligned_count`]="{ item }">
            {{ formatNumber(item.fully_aligned_count) }}
          </template>

          <template #[`item.spf_only_count`]="{ item }">
            {{ formatNumber(item.spf_aligned_count) }}
          </template>

          <template #[`item.dkim_only_count`]="{ item }">
            {{ formatNumber(item.dkim_aligned_count) }}
          </template>
        </v-data-table>
      </v-card-text>
    </v-card>

    <!-- Detail dialog component -->
    <DetailDialog
      v-model="detailDialog"
      :domain="domain"
      :source="selectedSource"
      :start-date="startDate || ''"
      :end-date="endDate || ''"
      :detail-data="dmarcStore.detailReport?.detail_rows || []"
      :loading="dmarcStore.loading"
      :error="dmarcStore.error || ''"
      @close="detailDialog = false"
    />
  </div>
</template>

<style lang="scss" scoped>
.report-view {
  // Page title styling
  .page-title {
    font-weight: 500;
    color: rgba(0, 0, 0, 0.87);
  }

  // Sources table custom styling
  .sources-table {
    // Make table rows clickable with hover effect
    :deep(tbody tr) {
      cursor: pointer;
      transition: background-color 0.2s;

      &:hover {
        background-color: rgba(0, 0, 0, 0.04);
      }
    }

    // Table header styling
    :deep(thead th) {
      font-weight: 600;
      color: rgba(0, 0, 0, 0.87);
    }

    // Responsive table adjustments
    @media (max-width: 960px) {
      font-size: 0.875rem;
    }
  }
}

// Card title with icon styling
:deep(.v-card-title) {
  .v-icon {
    color: rgba(0, 0, 0, 0.6);
  }
}
</style>
