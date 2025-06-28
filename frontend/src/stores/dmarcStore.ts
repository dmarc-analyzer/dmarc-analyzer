import type { DomainDetailResponse, DomainSummaryResponse } from '@/services/dmarcService'
import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import dmarcService from '@/services/dmarcService'

interface SummaryReport extends DomainSummaryResponse {
  chart_data?: {
    dates: string[]
    pass: number[]
    fail: number[]
  }
}

interface DetailReport extends DomainDetailResponse {}

export const useDmarcStore = defineStore('dmarc', () => {
  // State management for DMARC data
  const domains = ref<string[]>([])
  const summaryReport = ref<SummaryReport | null>(null)
  const detailReport = ref<DetailReport | null>(null)
  const loading = ref<boolean>(false)
  const error = ref<string | null>(null)

  // Computed properties

  /**
   * Transform summary report data into chart-compatible format
   * Returns chart data for visualization components
   */
  const chartData = computed(() => {
    if (!summaryReport.value || !summaryReport.value.chart_data) {
      return null
    }

    // Ensure chart_data has required data arrays
    if (!summaryReport.value.chart_data.dates || !summaryReport.value.chart_data.dates.length) {
      console.warn('Chart data missing dates array or empty dates array')
      return null
    }

    if (!summaryReport.value.chart_data.pass || !summaryReport.value.chart_data.fail) {
      console.warn('Chart data missing required data arrays')
      return null
    }

    // Return only Pass and Fail data lines, excluding Total
    return {
      labels: summaryReport.value.chart_data.dates,
      datasets: [
        {
          label: 'Pass',
          data: summaryReport.value.chart_data.pass,
          borderColor: '#4CAF50',
          backgroundColor: 'rgba(76, 175, 80, 0.1)',
          fill: true,
          pointRadius: 0,
          pointBorderWidth: 0,
          pointHoverRadius: 4,
          pointHoverBorderWidth: 4,
          pointHoverBackgroundColor: '#4CAF50',
        },
        {
          label: 'Fail',
          data: summaryReport.value.chart_data.fail,
          borderColor: '#F44336',
          backgroundColor: 'rgba(244, 67, 54, 0.1)',
          fill: true,
          pointRadius: 0,
          pointBorderWidth: 0,
          pointHoverRadius: 4,
          pointHoverBorderWidth: 4,
          pointHoverBackgroundColor: '#F44336',
        },
      ],
    }
  })

  // Actions

  /**
   * Fetch list of available domains from API
   * Provides mock data in development mode if API fails
   */
  const fetchDomains = async (): Promise<void> => {
    loading.value = true
    error.value = null

    try {
      const response = await dmarcService.getDomains()
      domains.value = response.data || []
    }
    catch (err: any) {
      error.value = `Failed to fetch domains: ${err.response?.data?.message || err.message}`
      console.error('Error fetching domains:', err)
    }
    finally {
      loading.value = false
    }
  }

  /**
   * Fetch summary report for a specific domain and date range
   * @param domain - Domain name to fetch report for
   * @param startDate - Optional start date (ISO format)
   * @param endDate - Optional end date (ISO format)
   */
  const fetchSummaryReport = async (domain: string, startDate?: string, endDate?: string): Promise<void> => {
    loading.value = true
    error.value = null
    // Don't clear existing data immediately to prevent UI flickering on refresh

    try {
      // Fetch report data from API
      const response = await dmarcService.getDomainSummary(domain, startDate, endDate)
      summaryReport.value = response.data

      // Fetch chart data separately
      try {
        const chartResponse = await dmarcService.getChartData(domain, startDate, endDate)

        if (!summaryReport.value) {
          summaryReport.value = {
            summary: [],
            domain_summary_counts: {
              total_count: 0,
              pass_count: 0,
              spf_aligned_count: 0,
              dkim_aligned_count: 0,
              fully_aligned_count: 0,
            },
            start_date: '',
            end_date: '',
            domain,
          }
        }

        // Process backend chart data response
        if (chartResponse.data && chartResponse.data.chartdata) {
          // Handle backend chartdata format
          const chartdata = chartResponse.data.chartdata
          const dates: string[] = []
          const pass: number[] = []
          const fail: number[] = []

          // Filter out potential total data, keep only pass and fail
          const filteredChartdata = chartdata.filter((item: any) => item.name === 'pass' || item.name === 'fail')

          // Ensure data exists and format is correct
          if (filteredChartdata && Array.isArray(filteredChartdata) && filteredChartdata.length >= 2
            && filteredChartdata[0].series && filteredChartdata[1].series
            && filteredChartdata[0].series.length === filteredChartdata[1].series.length) {
            // Determine which array is pass and which is fail
            const passIndex = filteredChartdata[0].name === 'pass' ? 0 : 1
            const failIndex = filteredChartdata[0].name === 'fail' ? 0 : 1

            // Extract dates and data points
            for (let i = 0; i < filteredChartdata[0].series.length; i++) {
              try {
                // Process date - could be timestamp (number) or date string
                const dateValue = filteredChartdata[0].series[i].name
                let dateStr: string

                if (typeof dateValue === 'number') {
                  // If timestamp (milliseconds)
                  dateStr = new Date(dateValue).toISOString().split('T')[0]
                }
                else {
                  // If date string
                  dateStr = new Date(dateValue).toISOString().split('T')[0]
                }

                dates.push(dateStr)

                // Get pass and fail values
                const passValue = Number(filteredChartdata[passIndex].series[i].value) || 0
                const failValue = Number(filteredChartdata[failIndex].series[i].value) || 0

                pass.push(passValue)
                fail.push(failValue)
              }
              catch (err) {
                console.error('Error processing chart data point:', err)
              }
            }

            // Ensure all arrays have data
            if (dates.length > 0 && pass.length === dates.length && fail.length === dates.length) {
              // Set chart data
              if (summaryReport.value) {
                summaryReport.value.chart_data = {
                  dates,
                  pass,
                  fail,
                }
              }
            }
            else {
              console.error('Failed to process chart data: arrays have different lengths')
            }
          }
          else {
            console.error('Invalid chart data format:', chartdata)
          }
        }
        else if (chartResponse.data && chartResponse.data.chart_data) {
          // If backend directly returns chart_data structure, use it
          if (summaryReport.value) {
            summaryReport.value.chart_data = chartResponse.data.chart_data
          }
        }
        else {
          // If backend returns other structure
          console.warn('Unexpected chart data format:', chartResponse.data)
          if (summaryReport.value && chartResponse.data) {
            // Try to cast the response data to chart_data format
            const responseData = chartResponse.data as any
            if (responseData.dates && responseData.pass && responseData.fail) {
              summaryReport.value.chart_data = {
                dates: responseData.dates,
                pass: responseData.pass,
                fail: responseData.fail,
              }
            }
          }
        }
      }
      catch (chartErr: any) {
        console.error('Error fetching chart data:', chartErr)
        // Chart data fetch failure doesn't affect overall report display
      }
    }
    catch (err: any) {
      error.value = `Failed to fetch summary report: ${err.response?.data?.message || err.message}`
      console.error('Error fetching summary report:', err)
    }
    finally {
      loading.value = false
    }
  }

  /**
   * Fetch detailed report for a specific domain and source
   * @param domain - Domain name
   * @param source - Source IP or hostname
   * @param startDate - Optional start date (ISO format)
   * @param endDate - Optional end date (ISO format)
   * @param sourceType - Optional source type filter
   */
  const fetchDetailReport = async (domain: string, source: string, startDate?: string, endDate?: string, sourceType: string = ''): Promise<void> => {
    loading.value = true
    error.value = null
    detailReport.value = null

    try {
      const response = await dmarcService.getDomainDetail(domain, source, startDate, endDate, sourceType)
      detailReport.value = response.data
    }
    catch (err: any) {
      error.value = `Failed to fetch detail report: ${err.response?.data?.message || err.message}`
      console.error('Error fetching detail report:', err)
    }
    finally {
      loading.value = false
    }
  }

  /**
   * Clear all report data and error states
   * Useful when navigating between different views
   */
  const clearReports = () => {
    summaryReport.value = null
    detailReport.value = null
    error.value = null
  }

  return {
    // State
    domains,
    summaryReport,
    detailReport,
    loading,
    error,

    // Computed
    chartData,

    // Actions
    fetchDomains,
    fetchSummaryReport,
    fetchDetailReport,
    clearReports,
  }
})
